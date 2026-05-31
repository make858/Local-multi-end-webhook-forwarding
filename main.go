package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

type Relay struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	SourcePath  string `json:"source_path"`
	TargetURL   string `json:"target_url"`
	TargetType  string `json:"target_type"`
	Token       string `json:"token"`
	Active      int    `json:"active"`
	CreatedAt   string `json:"created_at"`
	WebhookURL  string `json:"webhook_url,omitempty"`
	CustomTitle string `json:"custom_title"`
	CustomType  string `json:"custom_type"`
}

type Endpoint struct {
	ID           string `json:"id"`
	RelayID      string `json:"relay_id"`
	EndpointType string `json:"endpoint_type"`
	URL          string `json:"url"`
	Token        string `json:"token"`
	Active       int    `json:"active"`
	CreatedAt    string `json:"created_at"`
	SourcePath   string `json:"source_path"`
}

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Source    string `json:"source"`
}

type ReceivedWebhook struct {
	Timestamp string `json:"timestamp"`
	Source    string `json:"source"`
	Path      string `json:"path"`
	Body      string `json:"body"`
}

type SentWebhook struct {
	Timestamp string `json:"timestamp"`
	Target    string `json:"target"`
	Body      string `json:"body"`
	Success   bool   `json:"success"`
}

type RingBuffer struct {
	buffer []*LogEntry
	head   int
	tail   int
	size   int
	cap    int
	mu     sync.Mutex
}

type ReceivedBuffer struct {
	buffer []*ReceivedWebhook
	head   int
	tail   int
	size   int
	cap    int
	mu     sync.Mutex
}

type SentBuffer struct {
	buffer []*SentWebhook
	head   int
	tail   int
	size   int
	cap    int
	mu     sync.Mutex
}

type SSEClient struct {
	send chan []byte
}

var (
	db             *sql.DB
	ringBuffer     *RingBuffer
	receivedBuffer *ReceivedBuffer
	sentBuffer     *SentBuffer
	sseClients     = make(map[*SSEClient]bool)
	sseClientsMu   = sync.RWMutex{}
	maxDBSize      = int64(50 * 1024 * 1024) // 50MB
)

func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		buffer: make([]*LogEntry, capacity),
		cap:    capacity,
	}
}

func (rb *RingBuffer) Append(entry *LogEntry) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.buffer[rb.tail] = entry
	rb.tail = (rb.tail + 1) % rb.cap
	if rb.size < rb.cap {
		rb.size++
	} else {
		rb.head = (rb.head + 1) % rb.cap
	}
}

func (rb *RingBuffer) GetAll() []*LogEntry {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.size == 0 {
		return nil
	}

	result := make([]*LogEntry, 0, rb.size)
	if rb.tail > rb.head {
		result = append(result, rb.buffer[rb.head:rb.tail]...)
	} else {
		result = append(result, rb.buffer[rb.head:]...)
		result = append(result, rb.buffer[:rb.tail]...)
	}
	return result
}

func (rb *RingBuffer) GetRecent(count int) []*LogEntry {
	all := rb.GetAll()
	if len(all) <= count {
		return all
	}
	return all[len(all)-count:]
}

func NewReceivedBuffer(capacity int) *ReceivedBuffer {
	return &ReceivedBuffer{
		buffer: make([]*ReceivedWebhook, capacity),
		cap:    capacity,
	}
}

func (rb *ReceivedBuffer) Append(entry *ReceivedWebhook) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.buffer[rb.tail] = entry
	rb.tail = (rb.tail + 1) % rb.cap
	if rb.size < rb.cap {
		rb.size++
	} else {
		rb.head = (rb.head + 1) % rb.cap
	}
}

func (rb *ReceivedBuffer) GetAll() []*ReceivedWebhook {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.size == 0 {
		return nil
	}

	result := make([]*ReceivedWebhook, 0, rb.size)
	if rb.tail > rb.head {
		result = append(result, rb.buffer[rb.head:rb.tail]...)
	} else {
		result = append(result, rb.buffer[rb.head:]...)
		result = append(result, rb.buffer[:rb.tail]...)
	}
	return result
}

func (rb *ReceivedBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.head = 0
	rb.tail = 0
	rb.size = 0
}

func NewSentBuffer(capacity int) *SentBuffer {
	return &SentBuffer{
		buffer: make([]*SentWebhook, capacity),
		cap:    capacity,
	}
}

func (rb *SentBuffer) Append(entry *SentWebhook) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.buffer[rb.tail] = entry
	rb.tail = (rb.tail + 1) % rb.cap
	if rb.size < rb.cap {
		rb.size++
	} else {
		rb.head = (rb.head + 1) % rb.cap
	}
}

func (rb *SentBuffer) GetAll() []*SentWebhook {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.size == 0 {
		return nil
	}

	result := make([]*SentWebhook, 0, rb.size)
	if rb.tail > rb.head {
		result = append(result, rb.buffer[rb.head:rb.tail]...)
	} else {
		result = append(result, rb.buffer[rb.head:]...)
		result = append(result, rb.buffer[:rb.tail]...)
	}
	return result
}

func (rb *SentBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.head = 0
	rb.tail = 0
	rb.size = 0
}

func addLog(level, message, source string) {
	entry := &LogEntry{
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Level:     level,
		Message:   message,
		Source:    source,
	}
	ringBuffer.Append(entry)

	log.Printf("[%s] [%s] %s", source, level, message)

	go broadcastLog(entry)
}

func broadcastLog(entry *LogEntry) {
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	sseClientsMu.RLock()
	for client := range sseClients {
		select {
		case client.send <- data:
		default:
		}
	}
	sseClientsMu.RUnlock()
}

func columnExists(db *sql.DB, table, column string) bool {
	var count int
	err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = '%s'", table, column)).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

func tableExists(db *sql.DB, table string) bool {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?", table).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

func addColumnIfNotExists(db *sql.DB, table, column, colType string) {
	if !columnExists(db, table, column) {
		_, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, colType))
		if err != nil {
			log.Printf("警告: 添加 %s.%s 字段失败: %v\n", table, column, err)
		}
	}
}

func initDB() error {
	var err error

	execPath, err := os.Executable()
	var dbPath string
	if err == nil {
		dbDir := filepath.Dir(execPath)
		dbPath = filepath.Join(dbDir, "webhook_relay.db")
	} else {
		dbPath = "./webhook_relay.db"
	}

	log.Printf("数据库路径: %s\n", dbPath)

	db, err = sql.Open("sqlite", dbPath+"?cache=shared")
	if err != nil {
		return err
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	err = db.Ping()
	if err != nil {
		return err
	}

	log.Printf("数据库连接成功\n")

	createRelayTable := `
	CREATE TABLE IF NOT EXISTS relays (
		id TEXT PRIMARY KEY,
		name TEXT,
		source_path TEXT,
		target_url TEXT,
		target_type TEXT DEFAULT 'generic',
		token TEXT,
		active INTEGER DEFAULT 1,
		created_at TEXT DEFAULT CURRENT_TIMESTAMP,
		custom_title TEXT,
		custom_type TEXT
	);`
	_, err = db.Exec(createRelayTable)
	if err != nil {
		return err
	}

	addColumnIfNotExists(db, "relays", "custom_title", "TEXT")
	addColumnIfNotExists(db, "relays", "custom_type", "TEXT")

	createEndpointTable := `
	CREATE TABLE IF NOT EXISTS endpoints (
		id TEXT PRIMARY KEY,
		relay_id TEXT,
		endpoint_type TEXT,
		url TEXT,
		token TEXT,
		active INTEGER DEFAULT 1,
		created_at TEXT DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (relay_id) REFERENCES relays(id) ON DELETE CASCADE
	);`
	_, err = db.Exec(createEndpointTable)
	if err != nil {
		return err
	}

	addColumnIfNotExists(db, "endpoints", "source_path", "TEXT")

	if !tableExists(db, "users") {
		createUsersTable := `
			CREATE TABLE users (
				id TEXT PRIMARY KEY,
				username TEXT UNIQUE,
				password TEXT,
				created_at TEXT DEFAULT CURRENT_TIMESTAMP
			);`
		_, err = db.Exec(createUsersTable)
		if err != nil {
			return err
		}

		hashedPwd := hashPassword("admin123")
		_, err = db.Exec(`INSERT INTO users (id, username, password) VALUES (?, ?, ?)`, generateToken(), "admin", hashedPwd)
		if err != nil {
			log.Printf("警告: 创建默认用户失败: %v\n", err)
		} else {
			log.Printf("默认用户已创建: admin / admin123\n")
		}
	}

	return nil
}

func generateToken() string {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		return time.Now().Format("20060102150405")
	}
	return fmt.Sprintf("%x", b)
}

func getRelays() ([]*Relay, error) {
	rows, err := db.Query("SELECT id, name, source_path, target_url, target_type, token, active, created_at, custom_title, custom_type FROM relays ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relays []*Relay
	for rows.Next() {
		var r Relay
		var token sql.NullString
		var customTitle sql.NullString
		var customType sql.NullString
		err = rows.Scan(&r.ID, &r.Name, &r.SourcePath, &r.TargetURL, &r.TargetType, &token, &r.Active, &r.CreatedAt, &customTitle, &customType)
		if err != nil {
			return nil, err
		}
		if token.Valid {
			r.Token = token.String
		}
		if customTitle.Valid {
			r.CustomTitle = customTitle.String
		}
		if customType.Valid {
			r.CustomType = customType.String
		}
		if r.TargetType == "" {
			r.TargetType = "generic"
		}
		relays = append(relays, &r)
	}
	return relays, nil
}

func getRelay(id string) (*Relay, error) {
	var r Relay
	var token sql.NullString
	var customTitle sql.NullString
	var customType sql.NullString
	err := db.QueryRow("SELECT id, name, source_path, target_url, target_type, token, active, created_at, custom_title, custom_type FROM relays WHERE id = ?", id).
		Scan(&r.ID, &r.Name, &r.SourcePath, &r.TargetURL, &r.TargetType, &token, &r.Active, &r.CreatedAt, &customTitle, &customType)
	if err != nil {
		return nil, err
	}
	if token.Valid {
		r.Token = token.String
	}
	if customTitle.Valid {
		r.CustomTitle = customTitle.String
	}
	if customType.Valid {
		r.CustomType = customType.String
	}
	if r.TargetType == "" {
		r.TargetType = "generic"
	}
	return &r, nil
}

func addRelay(name, sourcePath, targetURL, targetType, token, customTitle, customType string) (string, error) {
	id := generateUUID()
	if token == "" {
		token = generateToken()
	}
	if targetType == "" {
		targetType = "generic"
	}
	_, err := db.Exec("INSERT INTO relays (id, name, source_path, target_url, target_type, token, custom_title, custom_type) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		id, name, sourcePath, targetURL, targetType, token, customTitle, customType)
	if err != nil {
		return "", err
	}
	addLog("INFO", fmt.Sprintf("创建中转规则: %s", name), "RELAY")
	return id, nil
}

func updateRelay(id, name, sourcePath, targetURL, targetType, token, customTitle, customType string, active int) error {
	if targetType == "" {
		targetType = "generic"
	}
	_, err := db.Exec("UPDATE relays SET name = ?, source_path = ?, target_url = ?, target_type = ?, token = ?, custom_title = ?, custom_type = ?, active = ? WHERE id = ?",
		name, sourcePath, targetURL, targetType, token, customTitle, customType, active, id)
	if err != nil {
		return err
	}
	addLog("INFO", fmt.Sprintf("更新中转规则: %s", name), "RELAY")
	return nil
}

func deleteRelay(id string) error {
	r, err := getRelay(id)
	if err == nil && r != nil {
		addLog("INFO", fmt.Sprintf("删除中转规则: %s", r.Name), "RELAY")
	}
	_, err = db.Exec("DELETE FROM endpoints WHERE relay_id = ?", id)
	if err != nil {
		return err
	}
	_, err = db.Exec("DELETE FROM relays WHERE id = ?", id)
	return err
}

func getEndpoints(relayID string) ([]*Endpoint, error) {
	var rows *sql.Rows
	var err error
	if relayID != "" {
		rows, err = db.Query("SELECT id, relay_id, endpoint_type, url, token, active, created_at, COALESCE(source_path, '') FROM endpoints WHERE relay_id = ?", relayID)
	} else {
		rows, err = db.Query("SELECT id, relay_id, endpoint_type, url, token, active, created_at, COALESCE(source_path, '') FROM endpoints")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var endpoints []*Endpoint
	for rows.Next() {
		var e Endpoint
		err = rows.Scan(&e.ID, &e.RelayID, &e.EndpointType, &e.URL, &e.Token, &e.Active, &e.CreatedAt, &e.SourcePath)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, &e)
	}
	return endpoints, nil
}

func addEndpoint(relayID, endpointType, url, token, sourcePath string) (string, error) {
	id := generateUUID()
	_, err := db.Exec("INSERT INTO endpoints (id, relay_id, endpoint_type, url, token, source_path) VALUES (?, ?, ?, ?, ?, ?)",
		id, relayID, endpointType, url, token, sourcePath)
	if err != nil {
		return "", err
	}
	addLog("INFO", fmt.Sprintf("添加端点: %s", endpointType), "ENDPOINT")
	return id, nil
}

func updateEndpoint(id, url, token string, active int, sourcePath string) error {
	_, err := db.Exec("UPDATE endpoints SET url = ?, token = ?, active = ?, source_path = ? WHERE id = ?", url, token, active, sourcePath, id)
	if err != nil {
		return err
	}
	addLog("INFO", fmt.Sprintf("更新端点状态: %s", map[bool]string{true: "启用", false: "禁用"}[active == 1]), "ENDPOINT")
	return nil
}

func deleteEndpoint(id string) error {
	addLog("INFO", "删除端点", "ENDPOINT")
	_, err := db.Exec("DELETE FROM endpoints WHERE id = ?", id)
	return err
}

func sendMattermost(webhookURL, message string) bool {
	// 步骤1: 发送一个最简单的测试消息，验证 webhook URL 是否正常工作
	testPayload := map[string]string{"text": "Webhook relay 收到消息"}
	testData, _ := json.Marshal(testPayload)
	addLog("DEBUG", fmt.Sprintf("Mattermost测试Payload: %s", string(testData)), "NOTIFICATION")
	addLog("DEBUG", fmt.Sprintf("Mattermost webhook URL: %s", webhookURL), "NOTIFICATION")

	client := &http.Client{Timeout: 10 * time.Second}

	testResp, testErr := client.Post(webhookURL, "application/json", strings.NewReader(string(testData)))
	if testErr != nil {
		addLog("ERROR", fmt.Sprintf("Mattermost测试发送失败: %s", testErr), "NOTIFICATION")
		// 即使测试失败，我们仍然尝试发送实际消息
	} else {
		testResp.Body.Close()
		addLog("DEBUG", fmt.Sprintf("Mattermost测试响应状态: %d", testResp.StatusCode), "NOTIFICATION")
	}

	// 步骤2: 清理并发送实际消息
	if message == "" {
		addLog("ERROR", "Mattermost发送失败: 消息为空", "NOTIFICATION")
		return false
	}

	// 创建一个安全的消息版本
	safeMsg := makeSafeText(message)
	payload := map[string]string{"text": safeMsg}
	data, err := json.Marshal(payload)
	if err != nil {
		addLog("ERROR", fmt.Sprintf("Mattermost发送失败: JSON编码错误: %s", err), "NOTIFICATION")
		return false
	}

	addLog("DEBUG", fmt.Sprintf("Mattermost实际Payload: %s", truncate(string(data), 300)), "NOTIFICATION")
	req, err := http.NewRequest("POST", webhookURL, strings.NewReader(string(data)))
	if err != nil {
		addLog("ERROR", fmt.Sprintf("Mattermost创建请求失败: %s", err), "NOTIFICATION")
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		addLog("ERROR", fmt.Sprintf("Mattermost发送失败: %s", err), "NOTIFICATION")
		return false
	}
	defer resp.Body.Close()

	success := resp.StatusCode == 200
	if success {
		addLog("INFO", "Mattermost发送成功", "NOTIFICATION")
	} else {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)
		addLog("ERROR", fmt.Sprintf("Mattermost发送失败: HTTP %d, 响应: %s", resp.StatusCode, truncate(bodyStr, 400)), "NOTIFICATION")
	}
	return success
}

func makeSafeText(s string) string {
	var result strings.Builder
	for _, r := range s {
		if r == 0 {
			continue
		}
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			continue
		}
		if r == 0xfffd {
			continue
		}
		if r > 0x10000 {
			continue
		}
		result.WriteRune(r)
	}
	resultStr := result.String()
	if resultStr == "" {
		return "[接收到空或无效消息]"
	}
	return resultStr
}

func sanitizeForJSON(s string) string {
	var result strings.Builder
	for _, r := range s {
		if r == 0 {
			continue
		}
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			continue
		}
		if r == 0xfffd {
			continue
		}
		result.WriteRune(r)
	}
	resultStr := result.String()
	if resultStr == "" {
		return "[无法解析的消息]"
	}
	return resultStr
}

func sendSynology(webhookURL, message string) bool {
	client := &http.Client{Timeout: 10 * time.Second}
	var req *http.Request
	var err error

	// 清理 URL 中的多余引号（群晖 Chat token 可能被错误地加上了引号）
	cleanURL := strings.ReplaceAll(webhookURL, "%22", "")
	cleanURL = strings.ReplaceAll(cleanURL, "%27", "") // 也处理单引号
	cleanURL = strings.ReplaceAll(cleanURL, "\"", "")
	cleanURL = strings.ReplaceAll(cleanURL, "'", "")

	addLog("DEBUG", fmt.Sprintf("群晖 Chat 原始 URL: %s", truncate(webhookURL, 100)), "NOTIFICATION")
	addLog("DEBUG", fmt.Sprintf("群晖 Chat 清理后 URL: %s", truncate(cleanURL, 100)), "NOTIFICATION")

	if strings.Contains(cleanURL, "text=") {
		encodedMsg := url.QueryEscape(message)
		finalURL := strings.Replace(cleanURL, "text=", "text="+encodedMsg, 1)
		req, err = http.NewRequest("GET", finalURL, nil)
	} else {
		// 群晖Chat需要使用表单数据格式：payload={"text": "消息"}
		textPayload := map[string]string{"text": message}
		textData, _ := json.Marshal(textPayload)
		formData := url.Values{}
		formData.Set("payload", string(textData))
		req, err = http.NewRequest("POST", cleanURL, strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	if err != nil {
		addLog("ERROR", fmt.Sprintf("群晖Chat发送失败: %s", err), "NOTIFICATION")
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		addLog("ERROR", fmt.Sprintf("群晖Chat发送失败: %s", err), "NOTIFICATION")
		return false
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// 群晖Chat API可能返回200但响应体包含错误信息
	success := resp.StatusCode == 200 && !strings.Contains(bodyStr, "error")

	if success {
		addLog("INFO", "群晖 Chat 发送成功", "NOTIFICATION")
	} else {
		addLog("ERROR", fmt.Sprintf("群晖 Chat 发送失败：HTTP %d, 响应：%s", resp.StatusCode, bodyStr), "NOTIFICATION")
	}
	return success
}

func sendGotify(webhookURL, message, token string) bool {
	finalURL := webhookURL
	if token != "" {
		finalURL = webhookURL + "/message?token=" + token
	}
	payload := map[string]interface{}{
		"title":    "Webhook中转通知",
		"message":  truncate(message, 1000),
		"priority": 5,
	}
	data, _ := json.Marshal(payload)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(finalURL, "application/json", strings.NewReader(string(data)))
	if err != nil {
		addLog("ERROR", fmt.Sprintf("Gotify发送失败: %s", err), "NOTIFICATION")
		return false
	}
	defer resp.Body.Close()
	success := resp.StatusCode == 200 || resp.StatusCode == 201
	addLog("INFO", fmt.Sprintf("Gotify%s", map[bool]string{true: "发送成功", false: fmt.Sprintf("发送失败: %d", resp.StatusCode)}[success]), "NOTIFICATION")
	return success
}

func sendGeneric(webhookURL, message string) bool {
	client := &http.Client{Timeout: 10 * time.Second}
	var req *http.Request
	var err error

	addLog("DEBUG", fmt.Sprintf("处理通用端点, URL: %s", truncate(webhookURL, 200)), "NOTIFICATION")
	addLog("DEBUG", fmt.Sprintf("包含 </webhook/>: %v, 包含 webhook?token=: %v, 包含 <文本内容>: %v",
		strings.Contains(webhookURL, "/webhook/"),
		strings.Contains(webhookURL, "webhook?token="),
		strings.Contains(webhookURL, "<文本内容>")), "NOTIFICATION")

	// 特殊处理：如果目标 URL 看起来是另一个 webhook-relay 的端点
	if (strings.Contains(webhookURL, "/webhook/") ||
		strings.Contains(webhookURL, "webhook?token=")) &&
		!strings.Contains(webhookURL, "<文本内容>") {
		addLog("DEBUG", fmt.Sprintf("检测到目标是另一个 webhook-relay 端点，发送原始消息"), "NOTIFICATION")
		// 直接发送原始消息作为 text/plain，不包装成 {"message": "..."}
		req, err = http.NewRequest("POST", webhookURL, strings.NewReader(message))
		req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	} else if strings.Contains(webhookURL, "<文本内容>") {
		// 正确替换 <文本内容> 占位符
		addLog("DEBUG", "检测到 <文本内容> 占位符", "NOTIFICATION")
		encodedMsg := url.QueryEscape(message)
		finalURL := strings.ReplaceAll(webhookURL, "<文本内容>", encodedMsg)
		addLog("DEBUG", fmt.Sprintf("替换后的 URL: %s", truncate(finalURL, 200)), "NOTIFICATION")
		req, err = http.NewRequest("GET", finalURL, nil)
	} else {
		// 使用完整消息，不截断
		payload := map[string]string{"message": message}
		data, _ := json.Marshal(payload)
		req, err = http.NewRequest("POST", webhookURL, strings.NewReader(string(data)))
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}

	if err != nil {
		addLog("ERROR", fmt.Sprintf("通用端点发送失败: %s", err), "NOTIFICATION")
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		addLog("ERROR", fmt.Sprintf("通用端点发送失败: %s", err), "NOTIFICATION")
		return false
	}
	defer resp.Body.Close()
	success := resp.StatusCode == 200 || resp.StatusCode == 201 || resp.StatusCode == 202
	addLog("INFO", fmt.Sprintf("通用端点%s", map[bool]string{true: "发送成功", false: fmt.Sprintf("发送失败: %d", resp.StatusCode)}[success]), "NOTIFICATION")
	return success
}

func sendToEndpointsWithData(relayID, message string, originalBody []byte, customTitle string) {
	addLog("INFO", fmt.Sprintf("开始发送到关联端点, relayID: %s, 消息长度: %d", relayID, len(message)), "NOTIFICATION")
	endpoints, err := getEndpoints(relayID)
	if err != nil {
		addLog("ERROR", fmt.Sprintf("获取端点失败: %s", err), "NOTIFICATION")
		return
	}
	addLog("INFO", fmt.Sprintf("找到 %d 个关联端点", len(endpoints)), "NOTIFICATION")
	for _, e := range endpoints {
		if e.Active == 1 {
			addLog("INFO", fmt.Sprintf("发送到端点: %s, 类型: %s", truncate(e.URL, 200), e.EndpointType), "NOTIFICATION")
			switch e.EndpointType {
			case "mattermost":
				go sendMattermost(e.URL, message)
			case "synology":
				go sendSynology(e.URL, message)
			case "gotify":
				go sendGotify(e.URL, message, e.Token)
			case "generic":
				// 检查目标是否是另一个 webhook-relay，但排除有 <文本内容> 的情况！
				if (strings.Contains(e.URL, "/webhook/") || strings.Contains(e.URL, "webhook?token=")) &&
					!strings.Contains(e.URL, "<文本内容>") {
					// 如果是，直接发送原始 JSON 数据，让对方自己处理
					go sendGenericRawJSON(e.URL, originalBody)
				} else {
					// 否则发送格式化后的消息（包括有 <文本内容> 占位符的）
					go sendGeneric(e.URL, message)
				}
			}
		}
	}
}

func sendToEndpoints(relayID, message string) {
	// 保持向后兼容
	sendToEndpointsWithData(relayID, message, nil, "")
}

func sendGenericRawJSON(webhookURL string, body []byte) bool {
	addLog("DEBUG", fmt.Sprintf("检测到目标是 webhook-relay，发送原始 JSON: %s", truncate(string(body), 100)), "NOTIFICATION")
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", webhookURL, strings.NewReader(string(body)))
	if err != nil {
		addLog("ERROR", fmt.Sprintf("发送原始 JSON 失败: %s", err), "NOTIFICATION")
		return false
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := client.Do(req)
	if err != nil {
		addLog("ERROR", fmt.Sprintf("发送原始 JSON 失败: %s", err), "NOTIFICATION")
		return false
	}
	defer resp.Body.Close()
	success := resp.StatusCode == 200 || resp.StatusCode == 201 || resp.StatusCode == 202
	addLog("INFO", fmt.Sprintf("原始 JSON 发送%s", map[bool]string{true: "成功", false: fmt.Sprintf("失败: %d", resp.StatusCode)}[success]), "NOTIFICATION")
	return success
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// beautifyMessage 将JSON格式消息转换为更易读的格式
func beautifyMessage(body []byte) string {
	return beautifyMessageWithCustomTitleAndType(body, "", "")
}

// beautifyMessageWithCustomTitleAndType 将JSON格式消息转换为更易读的格式，支持自定义标题和类型
func beautifyMessageWithCustomTitleAndType(body []byte, customTitle, customType string) string {
	if len(body) == 0 {
		return ""
	}

	// 检查是否已经是格式化好的文本（包含 emoji 等标记）
	bodyStr := string(body)
	if strings.Contains(bodyStr, "📌") ||
		strings.Contains(bodyStr, "📢") ||
		strings.Contains(bodyStr, "📄") ||
		strings.Contains(bodyStr, "💰") ||
		strings.Contains(bodyStr, "🕒") {
		return bodyStr
	}

	var jsonData map[string]interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		return bodyStr
	}

	if text, ok := jsonData["text"].(string); ok {
		return text
	}
	if message, ok := jsonData["message"].(string); ok {
		return message
	}

	title, _ := jsonData["title"].(string)
	content, _ := jsonData["content"].(string)
	values, _ := jsonData["values"].([]interface{})
	msgType, _ := jsonData["type"].(string)

	// 如果设置了自定义标题，使用自定义标题覆盖原始标题
	if customTitle != "" {
		title = customTitle
	}

	// 如果设置了自定义类型：
	// 1. 原始消息有 type 字段，则覆盖
	// 2. 原始消息没有 type 字段，则添加自定义类型
	if customType != "" {
		msgType = customType
	}

	if title != "" || content != "" || msgType != "" {
		result := ""
		// 添加类型（如果有）
		if msgType != "" {
			result += fmt.Sprintf("📌 类型：%s\n", msgType)
		}
		// 添加标题（如果有）
		if title != "" {
			result += fmt.Sprintf("📢 标题：%s\n", title)
		}
		// 添加内容（如果有），并替换占位符
		if content != "" {
			msg := content
			for i, v := range values {
				placeholder := fmt.Sprintf("{{value%d}}", i+1)
				if !strings.Contains(msg, placeholder) && i == 0 {
					placeholder = "{{value}}"
				}
				msg = strings.Replace(msg, placeholder, fmt.Sprintf("%v", v), -1)
			}
			result += fmt.Sprintf("📄 内容：%s\n", msg)
		}
		// 添加values的额外显示（如果有）
		if len(values) > 0 {
			for i, v := range values {
				valueLabel := fmt.Sprintf("💰 值%d", i+1)
				if i == 0 {
					valueLabel = "💰 剩余额度"
				}
				result += fmt.Sprintf("%s：%v\n", valueLabel, v)
			}
		}
		// 添加时间（如果有）
		if timestamp, ok := jsonData["timestamp"].(float64); ok {
			t := time.Unix(int64(timestamp), 0)
			result += fmt.Sprintf("🕒 时间：%s", t.Format("2006-01-02 15:04:05"))
		}

		return strings.TrimSpace(result)
	}

	// 当没有text/message/title/content时，将JSON转换为键值对的可读格式
	result := ""
	for k, v := range jsonData {
		result += fmt.Sprintf("%s: %v\n", k, v)
	}
	return strings.TrimSpace(result)
}

func generateUUID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return time.Now().Format("20060102150405.999999")
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func getServerAddresses() []string {
	var addresses []string

	hostname, err := os.Hostname()
	if err == nil {
		addresses = append(addresses, "主机名: "+hostname)
	}

	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip == nil || ip.IsLoopback() {
					continue
				}
				if ip.To4() != nil {
					addresses = append(addresses, fmt.Sprintf("IPv4: http://%s:5000", ip.String()))
				} else if ip.To16() != nil && !ip.IsLinkLocalUnicast() {
					addresses = append(addresses, fmt.Sprintf("IPv6: [%s]:5000", ip.String()))
				}
			}
		}
	}

	addresses = append(addresses, "本地: http://localhost:5000")
	addresses = append(addresses, "本地: http://127.0.0.1:5000")
	return addresses
}

func getMainIPv4() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		if localAddr.IP != nil && !localAddr.IP.IsLoopback() {
			return localAddr.IP.String()
		}
	}

	hostname, err := os.Hostname()
	if err == nil {
		addrs, err := net.LookupHost(hostname)
		if err == nil {
			for _, addr := range addrs {
				ip := net.ParseIP(addr)
				if ip != nil && !ip.IsLoopback() && ip.To4() != nil {
					return ip.String()
				}
			}
		}
	}

	return "localhost"
}

func sseHandler(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	client := &SSEClient{send: make(chan []byte, 256)}
	sseClientsMu.Lock()
	sseClients[client] = true
	sseClientsMu.Unlock()

	defer func() {
		sseClientsMu.Lock()
		delete(sseClients, client)
		sseClientsMu.Unlock()
		close(client.send)
	}()

	logs := ringBuffer.GetRecent(20)
	for _, logEntry := range logs {
		data, _ := json.Marshal(logEntry)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
	}
	c.Writer.Flush()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case data := <-client.send:
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			c.Writer.Flush()
		case <-ticker.C:
			fmt.Fprintf(c.Writer, ": keep-alive\n\n")
			c.Writer.Flush()
		case <-c.Request.Context().Done():
			return
		}
	}
}

var htmlContent []byte

func loadHTMLContent() {
	execPath, err := os.Executable()
	var baseDir string
	if err == nil {
		baseDir = filepath.Dir(execPath)
	} else {
		baseDir = "."
	}
	templatePath := filepath.Join(baseDir, "templates", "index.html")
	htmlContent, err = os.ReadFile(templatePath)
	if err != nil {
		panic(err)
	}
}

func webhookHandler(c *gin.Context) {
	sourcePath := c.Param("path")
	addLog("INFO", fmt.Sprintf("收到Webhook请求: %s /webhook/%s", c.Request.Method, sourcePath), "WEBHOOK")

	relays, err := getRelays()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		body = nil
	}
	c.Request.Body.Close()

	addLog("DEBUG", fmt.Sprintf("Webhook原始数据: %s", truncate(string(body), 500)), "WEBHOOK")

	receivedBuffer.Append(&ReceivedWebhook{
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Source:    "WEBHOOK",
		Path:      sourcePath,
		Body:      truncate(string(body), 2000),
	})

	for _, relay := range relays {
		if relay.Active == 1 && relay.SourcePath == sourcePath {
			if relay.Token != "" {
				token := c.Query("token")
				if token == "" {
					token = c.GetHeader("X-Token")
				}
				if token == "" {
					auth := c.GetHeader("Authorization")
					if strings.HasPrefix(auth, "Bearer ") {
						token = auth[7:]
					}
				}
				if token != relay.Token {
					addLog("ERROR", "令牌验证失败: 无效的令牌", "WEBHOOK")
					c.JSON(http.StatusForbidden, gin.H{"status": "error", "message": "无效的令牌"})
					return
				}
			}

			if relay.TargetURL == "" {
				addLog("ERROR", "目标URL无效: 空URL", "WEBHOOK")
				c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "目标URL无效"})
				return
			}

			addLog("INFO", fmt.Sprintf("转发到目标: %s (类型: %s)", truncate(relay.TargetURL, 50), relay.TargetType), "WEBHOOK")

			var success bool
			var respBody []byte

			switch relay.TargetType {
			case "mattermost":
				message := beautifyMessageWithCustomTitleAndType(body, relay.CustomTitle, relay.CustomType)
				if message == "" {
					message = string(body)
				}
				success = sendMattermost(relay.TargetURL, message)
				go sendToEndpointsWithData(relay.ID, message, body, relay.CustomTitle)
			case "synology":
				message := beautifyMessageWithCustomTitleAndType(body, relay.CustomTitle, relay.CustomType)
				if message == "" {
					message = string(body)
				}
				success = sendSynology(relay.TargetURL, message)
				go sendToEndpointsWithData(relay.ID, message, body, relay.CustomTitle)
			case "gotify":
				message := beautifyMessageWithCustomTitleAndType(body, relay.CustomTitle, relay.CustomType)
				if message == "" {
					message = string(body)
				}
				success = sendGotify(relay.TargetURL, message, relay.Token)
				go sendToEndpointsWithData(relay.ID, message, body, relay.CustomTitle)
			default:
				// 智能检测: 如果目标 URL 看起来像是 Mattermost webhook，则用 mattermost 方式发送
				if strings.Contains(relay.TargetURL, "/hooks/") || strings.Contains(relay.TargetURL, "mattermost") {
					addLog("DEBUG", "检测到 Mattermost 风格 webhook，使用 Mattermost 处理方式", "WEBHOOK")
					message := beautifyMessageWithCustomTitleAndType(body, relay.CustomTitle, relay.CustomType)
					if message == "" {
						message = string(body)
					}
					success = sendMattermost(relay.TargetURL, message)
					go sendToEndpointsWithData(relay.ID, message, body, relay.CustomTitle)
				} else {
					client := &http.Client{Timeout: 30 * time.Second}
					req, err := http.NewRequest(c.Request.Method, relay.TargetURL, strings.NewReader(string(body)))
					if err != nil {
						addLog("ERROR", fmt.Sprintf("转发失败: %s", err), "WEBHOOK")
						c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
						return
					}

					for k, v := range c.Request.Header {
						if k != "Host" {
							req.Header[k] = v
						}
					}

					resp, err := client.Do(req)
					if err != nil {
						addLog("ERROR", fmt.Sprintf("转发失败: %s", err), "WEBHOOK")
						c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
						return
					}
					defer resp.Body.Close()

					addLog("INFO", fmt.Sprintf("转发成功，状态码: %d", resp.StatusCode), "WEBHOOK")
					success = resp.StatusCode >= 200 && resp.StatusCode < 300

					respBody, err = io.ReadAll(resp.Body)
					if err == nil && len(respBody) > 0 {
						addLog("INFO", fmt.Sprintf("响应长度: %d字符", len(respBody)), "WEBHOOK")
						go sendToEndpointsWithData(relay.ID, string(respBody), respBody, "")
					}

					for k, v := range resp.Header {
						c.Header(k, v[0])
					}
					sentBuffer.Append(&SentWebhook{
						Timestamp: time.Now().Format("2006-01-02 15:04:05"),
						Target:    truncate(relay.TargetURL, 100),
						Body:      truncate(string(body), 2000),
						Success:   success,
					})
					c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
					return
				}
			}

			sentBuffer.Append(&SentWebhook{
				Timestamp: time.Now().Format("2006-01-02 15:04:05"),
				Target:    truncate(relay.TargetURL, 100),
				Body:      truncate(string(body), 2000),
				Success:   success,
			})

			if success {
				c.JSON(http.StatusOK, gin.H{"status": "success", "message": "转发成功"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "转发失败"})
			}
			return
		}
	}

	addLog("WARNING", fmt.Sprintf("未找到匹配的路由: %s", sourcePath), "WEBHOOK")
	c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "未找到匹配的webhook路由"})
}

func endpointWebhookHandler(c *gin.Context) {
	endpointPath := c.Param("endpointPath")
	addLog("INFO", fmt.Sprintf("收到端点Webhook请求: %s /direct/%s", c.Request.Method, endpointPath), "WEBHOOK")

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		body = nil
	}
	c.Request.Body.Close()

	addLog("DEBUG", fmt.Sprintf("Webhook原始数据: %s", truncate(string(body), 500)), "WEBHOOK")

	endpoints, err := getEndpoints("")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	addLog("DEBUG", fmt.Sprintf("共%d个端点，查找source_path=%s", len(endpoints), endpointPath), "WEBHOOK")

	for _, e := range endpoints {
		addLog("DEBUG", fmt.Sprintf("检查端点: id=%s, source_path=%s, active=%d", e.ID, e.SourcePath, e.Active), "WEBHOOK")
		if e.Active == 1 && e.SourcePath == endpointPath {
			if e.EndpointType == "" {
				addLog("ERROR", "端点类型无效", "WEBHOOK")
				c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "端点类型无效"})
				return
			}

			addLog("INFO", fmt.Sprintf("转发到端点: %s (类型: %s)", truncate(e.URL, 50), e.EndpointType), "WEBHOOK")

			var success bool
			switch e.EndpointType {
			case "mattermost":
				success = sendMattermost(e.URL, string(body))
			case "synology":
				success = sendSynology(e.URL, string(body))
			case "gotify":
				success = sendGotify(e.URL, string(body), e.Token)
			default:
				client := &http.Client{Timeout: 30 * time.Second}
				req, err := http.NewRequest(c.Request.Method, e.URL, strings.NewReader(string(body)))
				if err != nil {
					addLog("ERROR", fmt.Sprintf("转发失败: %s", err), "WEBHOOK")
					c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
					return
				}
				for k, v := range c.Request.Header {
					if k != "Host" {
						req.Header[k] = v
					}
				}
				resp, err := client.Do(req)
				if err != nil {
					addLog("ERROR", fmt.Sprintf("转发失败: %s", err), "WEBHOOK")
					c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
					return
				}
				defer resp.Body.Close()
				success = resp.StatusCode >= 200 && resp.StatusCode < 300
			}

			if success {
				c.JSON(http.StatusOK, gin.H{"status": "success", "message": "转发成功"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "转发失败"})
			}
			return
		}
	}

	addLog("WARNING", fmt.Sprintf("未找到匹配的端点: %s", endpointPath), "WEBHOOK")
	c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "未找到匹配的端点"})
}

type User struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Password  string `json:"-"`
	CreatedAt string `json:"created_at"`
}

var authenticatedUsers = sync.Map{}

func hashPassword(password string) string {
	b := []byte(password)
	h := make([]byte, 32)
	for i := 0; i < 32 && i < len(b); i++ {
		h[i] = b[i]
	}
	for i := 32; i < len(b); i++ {
		h[i%32] ^= b[i]
	}
	return fmt.Sprintf("%x", h)
}

func verifyPassword(username, password string) bool {
	var storedPassword string
	var id string
	err := db.QueryRow("SELECT id, password FROM users WHERE username = ?", username).Scan(&id, &storedPassword)
	if err != nil {
		return false
	}
	expectedHash := hashPassword(password)
	return expectedHash == storedPassword || storedPassword == password
}

func checkAndTrimDB() {
	fi, err := os.Stat("webhook-relay.db")
	if err != nil {
		return
	}

	if fi.Size() > maxDBSize {
		addLog("WARNING", fmt.Sprintf("数据库大小 %.2f MB 超过限制 %.2f MB，正在清理", float64(fi.Size())/1024/1024, float64(maxDBSize)/1024/1024), "SYSTEM")

		_, err = db.Exec("DELETE FROM relays WHERE created_at < datetime('now', '-30 days')")
		if err != nil {
			addLog("ERROR", fmt.Sprintf("清理旧规则失败: %s", err), "SYSTEM")
		}

		_, err = db.Exec("DELETE FROM endpoints WHERE created_at < datetime('now', '-30 days')")
		if err != nil {
			addLog("ERROR", fmt.Sprintf("清理旧端点失败: %s", err), "SYSTEM")
		}

		_, err = db.Exec("VACUUM")
		if err != nil {
			addLog("ERROR", fmt.Sprintf("VACUUM失败: %s", err), "SYSTEM")
		}

		addLog("INFO", "数据库清理完成", "SYSTEM")
	}
}

func main() {
	ringBuffer = NewRingBuffer(100)
	receivedBuffer = NewReceivedBuffer(50)
	sentBuffer = NewSentBuffer(50)

	err := initDB()
	if err != nil {
		log.Fatal(err)
	}

	checkAndTrimDB()

	loadHTMLContent()

	addLog("INFO", "Webhook中转服务启动", "SYSTEM")

	r := gin.Default()

	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}
		c.Next()
	})

	r.GET("/", func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")

		// 每次请求时重新读取模板文件，确保页面总是最新的
		execPath, err := os.Executable()
		var baseDir string
		if err == nil {
			baseDir = filepath.Dir(execPath)
		} else {
			baseDir = "."
		}
		templatePath := filepath.Join(baseDir, "templates", "index.html")
		content, err := os.ReadFile(templatePath)
		if err != nil {
			c.String(http.StatusInternalServerError, "加载模板失败: %v", err)
			return
		}
		c.Writer.Write(content)
	})

	r.POST("/login", func(c *gin.Context) {
		username := c.PostForm("username")
		password := c.PostForm("password")

		log.Printf("DEBUG - 登录请求: username=%s, password_length=%d", username, len(password))

		if username == "" || password == "" {
			log.Printf("DEBUG - 登录失败: 用户名或密码为空")
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "请输入用户名和密码"})
			return
		}

		if verifyPassword(username, password) {
			sessionToken := generateToken()
			authenticatedUsers.Store(sessionToken, username)
			c.SetCookie("session", sessionToken, 86400, "/", "", false, false)
			log.Printf("DEBUG - 登录成功: username=%s", username)
			c.JSON(http.StatusOK, gin.H{"status": "success", "message": "登录成功"})
			return
		}

		log.Printf("DEBUG - 登录失败: 用户名或密码错误")
		c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "用户名或密码错误"})
	})

	r.POST("/logout", func(c *gin.Context) {
		sessionToken, err := c.Cookie("session")
		if err == nil {
			authenticatedUsers.Delete(sessionToken)
		}
		c.SetCookie("session", "", -1, "/", "", false, true)
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "已登出"})
	})

	r.GET("/checkAuth", func(c *gin.Context) {
		sessionToken, err := c.Cookie("session")
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"authenticated": false})
			return
		}
		if username, ok := authenticatedUsers.Load(sessionToken); ok {
			c.JSON(http.StatusOK, gin.H{"authenticated": true, "username": username})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"authenticated": false})
	})

	r.POST("/changePassword", func(c *gin.Context) {
		sessionToken, err := c.Cookie("session")
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "未登录"})
			return
		}
		username, ok := authenticatedUsers.Load(sessionToken)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "未登录"})
			return
		}

		currentPassword := c.PostForm("currentPassword")
		newPassword := c.PostForm("newPassword")
		confirmPassword := c.PostForm("confirmPassword")

		if currentPassword == "" || newPassword == "" || confirmPassword == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "请填写所有字段"})
			return
		}

		if newPassword != confirmPassword {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "两次输入的新密码不一致"})
			return
		}

		if len(newPassword) < 6 {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "新密码长度至少6位"})
			return
		}

		var storedPassword string
		err = db.QueryRow("SELECT password FROM users WHERE username = ?", username).Scan(&storedPassword)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "查询用户失败"})
			return
		}

		expectedHash := hashPassword(currentPassword)
		if expectedHash != storedPassword && currentPassword != storedPassword {
			c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "当前密码错误"})
			return
		}

		hashedNewPassword := hashPassword(newPassword)
		_, err = db.Exec("UPDATE users SET password = ? WHERE username = ?", hashedNewPassword, username)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "修改密码失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "密码修改成功"})
	})

	r.GET("/status", func(c *gin.Context) {
		relays, err := getRelays()
		if err != nil {
			log.Printf("获取规则失败: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
			return
		}
		log.Printf("获取到 %d 条规则\n", len(relays))

		// 获取请求的协议和主机，使Webhook地址跟随浏览器地址
		scheme := "http"
		// 优先检查反向代理头部
		if forwardedProto := c.GetHeader("X-Forwarded-Proto"); forwardedProto == "https" {
			scheme = "https"
		} else if forwardedProto := c.GetHeader("X-Forwarded-Protocol"); forwardedProto == "https" {
			scheme = "https"
		} else if forwardedProto := c.GetHeader("X-Url-Scheme"); forwardedProto == "https" {
			scheme = "https"
		} else if forwardedSsl := c.GetHeader("X-Forwarded-Ssl"); forwardedSsl == "on" {
			scheme = "https"
		} else if frontEndHttps := c.GetHeader("Front-End-Https"); frontEndHttps == "on" {
			scheme = "https"
		} else if c.Request.TLS != nil {
			scheme = "https"
		}
		host := c.Request.Host

		for _, relay := range relays {
			if relay.Token != "" {
				relay.WebhookURL = fmt.Sprintf("%s://%s/webhook/%s?token=%s", scheme, host, relay.SourcePath, relay.Token)
			} else {
				relay.WebhookURL = fmt.Sprintf("%s://%s/webhook/%s", scheme, host, relay.SourcePath)
			}
		}
		c.JSON(http.StatusOK, relays)
	})

	r.POST("/relay", func(c *gin.Context) {
		name := c.PostForm("name")
		sourcePath := strings.Trim(c.PostForm("source_path"), "/")
		targetURL := c.PostForm("target_url")
		targetType := c.PostForm("target_type")
		token := c.PostForm("token")
		customTitle := c.PostForm("custom_title")
		customType := c.PostForm("custom_type")

		if name == "" || sourcePath == "" || targetURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "缺少必要参数"})
			return
		}

		id, err := addRelay(name, sourcePath, targetURL, targetType, token, customTitle, customType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "id": id})
	})

	r.PUT("/relay/:id", func(c *gin.Context) {
		id := c.Param("id")
		name := c.PostForm("name")
		sourcePath := strings.Trim(c.PostForm("source_path"), "/")
		targetURL := c.PostForm("target_url")
		targetType := c.PostForm("target_type")
		token := c.PostForm("token")
		customTitle := c.PostForm("custom_title")
		customType := c.PostForm("custom_type")
		active := 1
		if c.PostForm("active") == "0" {
			active = 0
		}

		if name == "" || sourcePath == "" || targetURL == "" {
			r, err := getRelay(id)
			if err == nil && r != nil {
				if name == "" {
					name = r.Name
				}
				if sourcePath == "" {
					sourcePath = r.SourcePath
				}
				if targetURL == "" {
					targetURL = r.TargetURL
				}
				if targetType == "" {
					targetType = r.TargetType
				}
				if token == "" {
					token = r.Token
				}
				if customTitle == "" {
					customTitle = r.CustomTitle
				}
				if customType == "" {
					customType = r.CustomType
				}
			}
		}

		err := updateRelay(id, name, sourcePath, targetURL, targetType, token, customTitle, customType, active)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	r.DELETE("/relay/:id", func(c *gin.Context) {
		id := c.Param("id")
		err := deleteRelay(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	r.GET("/endpoints/:relay_id", func(c *gin.Context) {
		relayID := c.Param("relay_id")
		endpoints, _ := getEndpoints(relayID)
		c.JSON(http.StatusOK, endpoints)
	})

	r.POST("/endpoint", func(c *gin.Context) {
		relayID := c.PostForm("relay_id")
		endpointType := c.PostForm("endpoint_type")
		url := c.PostForm("url")
		token := c.PostForm("token")
		sourcePath := c.PostForm("source_path")

		if relayID == "" || endpointType == "" || url == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "缺少必要参数"})
			return
		}

		id, err := addEndpoint(relayID, endpointType, url, token, sourcePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "id": id})
	})

	r.PUT("/endpoint/:id", func(c *gin.Context) {
		id := c.Param("id")
		url := c.PostForm("url")
		token := c.PostForm("token")
		sourcePath := c.PostForm("source_path")
		active := 1
		if c.PostForm("active") == "0" {
			active = 0
		}

		err := updateEndpoint(id, url, token, active, sourcePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	r.DELETE("/endpoint/:id", func(c *gin.Context) {
		id := c.Param("id")
		err := deleteEndpoint(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	r.POST("/endpoint/test", func(c *gin.Context) {
		endpointType := c.PostForm("endpoint_type")
		url := c.PostForm("url")
		token := c.PostForm("token")

		if endpointType == "" || url == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "缺少必要参数"})
			return
		}

		testMessage := "这是一条测试消息，用于验证Webhook连接是否正常。"
		var success bool
		switch endpointType {
		case "mattermost":
			success = sendMattermost(url, testMessage)
		case "synology":
			success = sendSynology(url, testMessage)
		case "gotify":
			success = sendGotify(url, testMessage, token)
		case "generic":
			success = sendGeneric(url, testMessage)
		default:
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "未知的端点类型"})
			return
		}

		if success {
			c.JSON(http.StatusOK, gin.H{"status": "success", "message": "测试消息发送成功"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "测试消息发送失败，请检查URL和配置"})
		}
	})

	r.GET("/logs", func(c *gin.Context) {
		count := 50
		if c.Query("count") != "" {
			var err error
			count, err = strconv.Atoi(c.Query("count"))
			if err != nil || count > 100 {
				count = 50
			}
		}
		logs := ringBuffer.GetRecent(count)
		c.JSON(http.StatusOK, logs)
	})

	r.GET("/addresses", func(c *gin.Context) {
		addresses := getServerAddresses()
		c.JSON(http.StatusOK, addresses)
	})

	r.GET("/logs/sse", sseHandler)

	r.POST("/test-form", func(c *gin.Context) {
		body, _ := io.ReadAll(c.Request.Body)
		c.JSON(http.StatusOK, gin.H{
			"form_data": c.PostFormMap(""),
			"token":     c.PostForm("token"),
			"data":      truncate(string(body), 200),
		})
	})

	r.GET("/webhook/received", func(c *gin.Context) {
		c.JSON(http.StatusOK, receivedBuffer.GetAll())
	})

	r.DELETE("/webhook/received", func(c *gin.Context) {
		receivedBuffer.Clear()
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	r.GET("/webhook/sent", func(c *gin.Context) {
		c.JSON(http.StatusOK, sentBuffer.GetAll())
	})

	r.DELETE("/webhook/sent", func(c *gin.Context) {
		sentBuffer.Clear()
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	r.POST("/webhook/:path", webhookHandler)
	r.GET("/webhook/:path", webhookHandler)
	r.PUT("/webhook/:path", webhookHandler)
	r.DELETE("/webhook/:path", webhookHandler)

	r.GET("/direct/:endpointPath", endpointWebhookHandler)
	r.POST("/direct/:endpointPath", endpointWebhookHandler)
	r.PUT("/direct/:endpointPath", endpointWebhookHandler)
	r.DELETE("/direct/:endpointPath", endpointWebhookHandler)

	r.Run("[::]:5000")
}
