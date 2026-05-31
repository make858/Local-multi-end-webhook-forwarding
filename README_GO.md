# Webhook Relay (Go 版本)

基于 Go 重写的 Webhook 中转工具，功能完全与原 Python 版本一致。

## 功能特性

- ✅ **Webhook 中转** - 接收 HTTP 请求并转发到目标 URL
- ✅ **多端接收** - 支持同时转发到多个通知服务：
  - Mattermost
  - 群晖 Chat
  - Gotify
  - 通用 HTTP 端点
- ✅ **可视化管理** - 提供 Web 界面管理中转规则
- ✅ **数据持久化** - 使用 SQLite 保存配置
- ✅ **状态控制** - 支持启用/禁用规则和端点
- ✅ **实时日志** - 使用 SSE (Server-Sent Events) 推送实时操作日志
- ✅ **性能优化** - 高性能 Go 语言实现，并发安全

## 快速开始

### 运行程序

**Windows:**
```bash
start.bat
```

**或直接运行:**
```bash
go run main.go
```

### 编译构建

**Windows:**
```bash
go build -o webhook-relay.exe
```

**Linux/Mac:**
```bash
go build -o webhook-relay
```

### 访问地址

启动后访问: `http://localhost:5000`

## 项目结构

```
webhook-relay/
├── main.go           # 主程序文件 (Go 版本)
├── go.mod            # Go 模块依赖
├── templates/
│   └── index.html    # Web 界面模板
├── webhook_relay.db  # SQLite 数据库 (自动生成)
├── start.bat         # Windows 启动脚本
├── start.sh          # Linux 启动脚本 (原 Python 版本)
└── README_GO.md      # Go 版本说明文档
```

## 与原 Python 版本对比

| 特性 | Python 版本 | Go 版本 |
|------|------------|---------|
| 性能 | 中等 | 高 |
| 内存占用 | 较高 | 较低 |
| 部署 | 需要 Python 环境 | 单二进制文件 |
| 并发性 | 多线程/进程 | 原生 Goroutine |
| SSE 支持 | 支持 | 支持 |
| SQLite | 支持 | 支持 |
| API 接口 | 完全一致 | 完全一致 |
| Web 界面 | 完全一致 | 完全一致 |

## API 接口

与原 Python 版本完全兼容，使用方式没有变化：

- `GET /` - 首页，显示所有规则
- `GET /status` - 获取所有规则列表
- `POST /relay` - 创建中转规则
- `PUT /relay/:id` - 更新中转规则
- `DELETE /relay/:id` - 删除中转规则
- `GET /endpoints/:relay_id` - 获取规则的端点列表
- `POST /endpoint` - 创建端点
- `PUT /endpoint/:id` - 更新端点
- `DELETE /endpoint/:id` - 删除端点
- `ANY /webhook/:path` - 接收并转发 Webhook
- `GET /logs` - 获取操作日志
- `GET /logs/sse` - SSE 实时日志推送

## 使用示例

添加中转规则、多端端点等操作完全与原 Python 版本一致，详细示例可参考原 README.md。

## 技术栈

- **Web 框架**: Gin (高性能 HTTP 框架)
- **数据库**: SQLite (github.com/mattn/go-sqlite3)
- **模板引擎**: html/template (标准库)

## 注意事项

- 数据库文件位置与原版本保持一致 (`webhook_relay.db`)
- 所有 API 接口保持完全兼容，可直接替换部署
- 启动后监听 `0.0.0.0:5000`，与原版本一致
