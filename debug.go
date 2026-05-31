package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	fmt.Println("开始调试...")
	
	execPath, err := os.Executable()
	if err != nil {
		fmt.Printf("获取可执行文件路径失败: %v\n", err)
	} else {
		fmt.Printf("可执行文件路径: %s\n", execPath)
		fmt.Printf("可执行文件目录: %s\n", filepath.Dir(execPath))
	}
	
	var baseDir string
	if err == nil {
		baseDir = filepath.Dir(execPath)
	} else {
		baseDir = "."
	}
	fmt.Printf("基础目录: %s\n", baseDir)
	
	templatePath := filepath.Join(baseDir, "templates", "index.html")
	fmt.Printf("模板路径: %s\n", templatePath)
	
	content, err := os.ReadFile(templatePath)
	if err != nil {
		fmt.Printf("读取模板失败: %v\n", err)
	} else {
		fmt.Printf("读取模板成功，长度: %d\n", len(content))
	}
	
	dbPath := filepath.Join(baseDir, "webhook_relay.db")
	fmt.Printf("数据库路径: %s\n", dbPath)
	
	dbStat, err := os.Stat(dbPath)
	if err != nil {
		fmt.Printf("数据库文件不存在: %v\n", err)
	} else {
		fmt.Printf("数据库文件存在，大小: %d\n", dbStat.Size())
	}
}
