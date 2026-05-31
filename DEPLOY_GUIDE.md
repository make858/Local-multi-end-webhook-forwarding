# Webhook-Relay 已编译程序运行说明文档

---

## 一、程序概述

`webhook-relay` 是一个基于 Go 语言开发的 Webhook 中转服务，支持将收到的 HTTP 请求转发到多个目标端点（Mattermost、群晖 Chat、Gotify、通用 HTTP）。

### 功能特性

| 特性 | 说明 |
| --- | --- |
| 多端支持 | Mattermost、群晖 Chat、Gotify、通用 HTTP |
| 可视化管理 | Web 界面管理中转规则 |
| 数据持久化 | SQLite 数据库存储 |
| 实时日志 | SSE 推送实时操作日志 |
| 令牌验证 | 支持 token 参数验证 |

---

## 二、已编译文件说明

项目中已包含以下预编译的二进制文件：

| 文件 | 平台 | 架构 |
| --- | --- | --- |
| `webhook-relay` | Linux | x86_64 |
| `webhook-relay.exe` | Windows | x86_64 |

### 文件结构

```
webhook-relay/
├── webhook-relay          # Linux 可执行文件
├── webhook-relay.exe      # Windows 可执行文件
├── templates/
│   └── index.html         # Web 界面模板（必需）
└── webhook_relay.db       # SQLite 数据库（首次运行自动创建）
```

> **注意**：`templates/` 目录必须与可执行文件在同一目录下。

---

## 三、快速运行指南

### 3.1 Linux 系统

#### 方式一：直接运行（前台）

```bash
# 进入程序目录
cd /opt/webhook-relay

# 赋予执行权限
chmod +x webhook-relay

# 运行程序
./webhook-relay
```

#### 方式二：后台运行

```bash
# 使用 nohup 后台运行
nohup ./webhook-relay > /dev/null 2>&1 &

# 或使用 screen
screen -S webhook-relay ./webhook-relay
```

#### 方式三：指定端口运行

```bash
# 默认端口为 5000，可通过环境变量指定
PORT=8080 ./webhook-relay
```

### 3.2 Windows 系统

#### 方式一：双击运行

直接双击 `webhook-relay.exe` 文件，会打开一个命令行窗口显示运行日志。

#### 方式二：命令行运行

```cmd
:: 进入程序目录
cd C:\webhook-relay

:: 运行程序
webhook-relay.exe

:: 指定端口
set PORT=8080
webhook-relay.exe
```

#### 方式三：后台运行（无窗口）

创建 `start.vbs` 文件：

```vbscript
Set WshShell = CreateObject("WScript.Shell")
WshShell.Run "C:\webhook-relay\webhook-relay.exe", 0, False
```

双击 `start.vbs` 即可在后台运行。

---

## 四、服务化部署

### 4.1 Linux systemd 服务

**步骤 1：创建服务配置文件**

```bash
cat > /etc/systemd/system/webhook-relay.service << 'EOF'
[Unit]
Description=Webhook Relay Service
After=network.target

[Service]
Type=simple
User=nobody
Group=nogroup
WorkingDirectory=/opt/webhook-relay
ExecStart=/opt/webhook-relay/webhook-relay
Restart=always
RestartSec=5
Environment="PORT=5000"

[Install]
WantedBy=multi-user.target
EOF
```

**步骤 2：配置权限**

```bash
# 创建程序目录
mkdir -p /opt/webhook-relay

# 复制程序文件
cp webhook-relay /opt/webhook-relay/
cp -r templates /opt/webhook-relay/

# 设置权限
chown -R nobody:nogroup /opt/webhook-relay
chmod 755 /opt/webhook-relay/webhook-relay
```

**步骤 3：启动服务**

```bash
# 重新加载 systemd
systemctl daemon-reload

# 启动服务
systemctl start webhook-relay

# 设置开机自启
systemctl enable webhook-relay

# 查看状态
systemctl status webhook-relay
```

### 4.2 Alpine Linux OpenRC 服务

```bash
cat > /etc/init.d/webhook-relay << 'EOF'
#!/sbin/openrc-run

name="webhook-relay"
description="Webhook Relay Service"

command="/opt/webhook-relay/webhook-relay"
command_background="yes"
command_user="nobody"

pidfile="/run/webhook-relay.pid"
directory="/opt/webhook-relay"

depend() {
    need net
    after firewall
}

start_pre() {
    checkpath --directory --owner nobody:nogroup /run/webhook-relay
}
EOF

# 设置权限
chmod +x /etc/init.d/webhook-relay

# 添加到开机自启
rc-update add webhook-relay default

# 启动服务
rc-service webhook-relay start
```

### 4.3 Windows 服务（使用 NSSM）

**步骤 1：安装 NSSM**

```cmd
choco install nssm -y
```

**步骤 2：创建服务**

```cmd
nssm install webhook-relay
```

在弹出的窗口中配置：

- **Path**: `C:\webhook-relay\webhook-relay.exe`
- **Working Directory**: `C:\webhook-relay`

**步骤 3：启动服务**

```cmd
net start webhook-relay
```

---

## 五、配置说明

### 5.1 环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PORT` | 5000 | 服务监听端口 |
| `DB_PATH` | `webhook_relay.db` | 数据库文件路径 |
| `LOG_LEVEL` | INFO | 日志级别（DEBUG/INFO/WARN/ERROR） |

### 5.2 数据库文件

程序首次运行时会自动创建 `webhook_relay.db` 数据库文件，包含以下表：

| 表名 | 用途 |
| --- | --- |
| `relays` | 中转规则配置 |
| `endpoints` | 多端端点配置 |

---

## 六、访问服务

### 6.1 Web 管理界面

启动后访问：`http://<服务器IP>:5000`

### 6.2 API 接口

| 接口 | 方法 | 说明 |
| --- | --- | --- |
| `/` | GET | Web 管理界面 |
| `/status` | GET | 获取规则状态（JSON） |
| `/webhook/{path}` | ANY | 接收 Webhook 并中转 |
| `/logs` | GET | 获取操作日志 |
| `/logs/sse` | GET | SSE 实时日志推送 |

### 6.3 使用示例

```bash
# 添加中转规则
curl -X POST http://localhost:5000/relay \
  -d "name=GitHub Webhook" \
  -d "source_path=github" \
  -d "target_url=https://mattermost.example.com/hooks/xxx" \
  -d "target_type=mattermost"

# 测试 Webhook
curl -X POST http://localhost:5000/webhook/github \
  -H "Content-Type: application/json" \
  -d '{"event": "push", "message": "Hello World"}'
```

---

## 七、防火墙配置

### 7.1 Linux iptables

```bash
# 开放 5000 端口
iptables -A INPUT -p tcp --dport 5000 -j ACCEPT
iptables-save > /etc/iptables/rules-save
```

### 7.2 Linux ufw

```bash
ufw allow 5000/tcp
ufw reload
```

### 7.3 Windows 防火墙

```powershell
New-NetFirewallRule -DisplayName "Webhook Relay" -Direction Inbound -Protocol TCP -LocalPort 5000 -Action Allow
```

---

## 八、故障排除

### 8.1 常见问题

| 问题 | 原因 | 解决方案 |
| --- | --- | --- |
| 端口被占用 | 5000 端口已被其他服务使用 | 修改 `PORT` 环境变量 |
| 启动失败 | 权限不足 | 检查目录和文件权限 |
| 无法访问界面 | 防火墙阻止 | 开放 5000 端口 |
| 数据库错误 | 数据库文件权限问题 | 设置正确的文件所有权 |

### 8.2 查看日志

```bash
# systemd 日志
journalctl -u webhook-relay -f

# OpenRC 日志
cat /var/log/messages | grep webhook-relay
```

### 8.3 检查服务状态

```bash
# systemd
systemctl status webhook-relay

# OpenRC  
rc-service webhook-relay status

# 检查端口
netstat -tlnp | grep 5000
ss -tlnp | grep 5000
```

---

## 九、性能优化建议

### 9.1 资源限制

在 `/etc/security/limits.conf` 中添加：

```
nobody soft nofile 1024
nobody hard nofile 4096
```

### 9.2 反向代理（推荐）

使用 Nginx 作为反向代理：

```nginx
server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://localhost:5000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

---

## 十、备份与恢复

### 10.1 备份数据库

```bash
# 备份数据库
cp /opt/webhook-relay/webhook_relay.db /opt/webhook-relay/webhook_relay.db.backup

# 备份整个目录
tar -czf webhook-relay-backup.tar.gz /opt/webhook-relay/
```

### 10.2 恢复数据库

```bash
# 停止服务
systemctl stop webhook-relay

# 恢复数据库
cp /opt/webhook-relay/webhook_relay.db.backup /opt/webhook-relay/webhook_relay.db

# 启动服务
systemctl start webhook-relay
```

---

## 十一、版本更新

### 11.1 更新步骤

```bash
# 停止服务
systemctl stop webhook-relay

# 备份现有文件
cp /opt/webhook-relay/webhook-relay /opt/webhook-relay/webhook-relay.old

# 复制新的可执行文件
cp new-webhook-relay /opt/webhook-relay/webhook-relay

# 启动服务
systemctl start webhook-relay
```

---

## 附录：编译说明

如果需要自行编译，确保已安装 Go 1.21+：

```bash
# 编译 Linux 版本
GOOS=linux GOARCH=amd64 go build -o webhook-relay main.go

# 编译 Windows 版本
GOOS=windows GOARCH=amd64 go build -o webhook-relay.exe main.go

# 编译 macOS 版本
GOOS=darwin GOARCH=amd64 go build -o webhook-relay-darwin main.go
```

---

**文档版本**: v1.0\
**更新日期**: 2026年5月\
**适用程序版本**: webhook-relay (Go 版本)