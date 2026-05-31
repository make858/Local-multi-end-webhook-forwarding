# Webhook 中转工具

一个基于 Go (Gin 框架) 的 Webhook 中转工具，支持多端接收（Mattermost、群晖 Chat、Gotify、通用 HTTP），使用 SQLite 持久化存储配置。

[https://github.com/make858/Local-multi-end-webhook-forwarding/blob/main/1.png](https://)

[https://github.com/make858/Local-multi-end-webhook-forwarding/blob/main/2.png](https://)

[https://github.com/make858/Local-multi-end-webhook-forwarding/blob/main/3.png](https://)

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
- ✅ **实时日志** - 使用 SSE（Server-Sent Events）推送实时操作日志
- ✅ **卡片式美化** - 端点列表和中转规则使用响应式卡片布局
- ✅ **自定义类型/标题** - 支持覆盖原始消息的 type 和 title 字段
- ✅ **IPv6 支持** - 同时支持 IPv4 和 IPv6 连接
- ✅ **用户认证** - 支持用户名密码登录，保护管理界面
- ✅ **Webhook 监控** - 记录收到和发送的 Webhook 消息
- ✅ **端点直接访问** - 支持独立访问每个端点的 Webhook 地址
- ✅ **数据库限制** - 自动限制数据库大小为 50MB
- ✅ **HTTPS 自动检测** - 支持反向代理场景下的协议自动检测

## 快速开始

### 环境要求

- Go 1.21+
- SQLite3

### 编译运行

```bash
# 克隆代码
git clone <repository_url>
cd webhook-relay

# 编译
go build -o webhook-relay main.go

# 运行
./webhook-relay
```

### 访问地址

启动后访问：`http://localhost:5000`

### 默认账号

- 用户名：`admin`
- 密码：`admin123`

## 支持的端点类型


| 类型       | 说明               | 配置说明                              |
| ---------- | ------------------ | ------------------------------------- |
| mattermost | Mattermost Webhook | URL: Mattermost Webhook 地址          |
| synology   | 群晖 Chat          | URL: 群晖 Chat Webhook 地址           |
| gotify     | Gotify 消息推送    | URL: Gotify 地址, Token: Gotify Token |
| generic    | 通用 HTTP          | URL: 目标地址                         |

## Web 界面功能

### 标签页说明


| 标签        | 说明                        |
| ----------- | --------------------------- |
| 中转规则    | 管理 Webhook 中转规则       |
| 操作日志    | 实时查看系统操作日志        |
| 收到Webhook | 查看所有收到的 Webhook 请求 |
| 发送Webhook | 查看所有转发的 Webhook 消息 |

### 中转规则功能

- **添加规则** - 创建新的中转规则
- **编辑规则** - 修改现有规则配置
- **启用/禁用** - 控制规则是否生效
- **配置多端** - 设置多个目标端点
- **复制 Webhook 地址** - 一键复制访问地址

### 端点直接访问

每个端点可以设置独立的访问路径，支持直接发送 Webhook 到指定端点：

- 访问格式：`http://IP:5000/direct/{端点路径}`
- 适用于：绕过中转规则，直接发送到特定端点

## API 接口

### 用户认证


| 方法 | 路径              | 说明         |
| ---- | ----------------- | ------------ |
| POST | `/login`          | 用户登录     |
| POST | `/logout`         | 用户登出     |
| GET  | `/checkAuth`      | 检查登录状态 |
| POST | `/changePassword` | 修改密码     |

### 中转规则


| 方法   | 路径         | 说明                     |
| ------ | ------------ | ------------------------ |
| GET    | `/status`    | 获取所有规则列表（JSON） |
| POST   | `/relay`     | 创建中转规则             |
| PUT    | `/relay/:id` | 更新中转规则             |
| DELETE | `/relay/:id` | 删除中转规则             |

### 多端端点


| 方法   | 路径                   | 说明               |
| ------ | ---------------------- | ------------------ |
| GET    | `/endpoints/:relay_id` | 获取规则的端点列表 |
| POST   | `/endpoint`            | 创建端点           |
| PUT    | `/endpoint/:id`        | 更新端点           |
| DELETE | `/endpoint/:id`        | 删除端点           |
| POST   | `/endpoint/test`       | 测试端点连接       |

### Webhook 接收


| 方法 | 路径                    | 说明                |
| ---- | ----------------------- | ------------------- |
| ANY  | `/webhook/:path`        | 接收 Webhook 并中转 |
| ANY  | `/direct/:endpointPath` | 端点直接访问        |

### Webhook 监控


| 方法   | 路径                | 说明               |
| ------ | ------------------- | ------------------ |
| GET    | `/webhook/received` | 获取收到的消息列表 |
| DELETE | `/webhook/received` | 清空收到的消息     |
| GET    | `/webhook/sent`     | 获取发送的消息列表 |
| DELETE | `/webhook/sent`     | 清空发送的消息     |

### 系统接口


| 方法 | 路径         | 说明               |
| ---- | ------------ | ------------------ |
| GET  | `/logs`      | 获取最近的操作日志 |
| GET  | `/logs/sse`  | SSE 实时日志推送   |
| GET  | `/addresses` | 获取服务器地址列表 |

## 使用示例

### 用户认证

```bash
# 登录
curl -X POST http://localhost:5000/login \
  -d "username=admin&password=admin123"

# 修改密码
curl -X POST http://localhost:5000/changePassword \
  -d "currentPassword=admin123&newPassword=newpass123&confirmPassword=newpass123"
```

### 添加中转规则

```bash
curl -X POST http://localhost:5000/relay \
  -d "name=GitHub Webhook" \
  -d "source_path=github" \
  -d "target_url=https://api.example.com/webhook"
```

### 添加多端端点

```bash
curl -X POST http://localhost:5000/endpoint \
  -d "relay_id=xxx" \
  -d "endpoint_type=mattermost" \
  -d "url=https://mattermost.example.com/hooks/xxx" \
  -d "source_path=myendpoint"
```

### 调用 Webhook

```bash
# 通过中转规则
curl -X POST http://localhost:5000/webhook/github \
  -H "Content-Type: application/json" \
  -d '{"event": "push", "message": "Hello World"}'

# 直接访问端点
curl -X POST http://localhost:5000/direct/myendpoint \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello"}'
```

## 项目结构

```
webhook-relay/
├── main.go              # 主应用程序 (Go)
├── templates/
│   └── index.html       # Web 界面模板
├── webhook_relay.db     # SQLite 数据库（自动生成）
├── README.md            # 说明文档
└── DEPLOY_*.md          # 部署指南
```

---

## 部署指南

### 目录

- [Alpine Linux 部署 (OpenRC)](#alpine-linux-%E9%83%A8%E7%BD%B2)
- [Ubuntu/Debian 部署 (Systemd)](#ubuntudebian-%E9%83%A8%E7%BD%B2)
- [CentOS/RHEL 部署 (Systemd)](#centosrhel-%E9%83%A8%E7%BD%B2)
- [Docker 部署](#docker-%E9%83%A8%E7%BD%B2)
- [群晖 NAS (Synology) 部署](#%E7%BE%A4%E6%99%96-nas-%E9%83%A8%E7%BD%B2)

---

## Alpine Linux 部署

### 一、系统准备

```bash
# 更新系统
apk update && apk upgrade

# 安装必要依赖
apk add go sqlite git
```

### 二、创建项目目录

```bash
mkdir -p /opt/webhook-relay
cd /opt/webhook-relay
```

### 三、编译项目

```bash
# 克隆或复制代码
# 如果使用 git
git clone <repository_url> .

# 编译
go build -o webhook-relay main.go
```

### 四、配置 OpenRC 服务

创建服务配置文件：

```bash
cat > /etc/init.d/webhook-relay << 'EOF'
#!/sbin/openrc-run

name="webhook-relay"
description="Webhook Relay Service"

command="/opt/webhook-relay/webhook-relay"
command_background="yes"
command_user="root"

pidfile="/run/webhook-relay.pid"
directory="/opt/webhook-relay"

depend() {
    need net
    after firewall
}

start_pre() {
    checkpath --directory --owner root:root /run/webhook-relay
}
EOF
```

### 五、启动服务

```bash
# 赋予执行权限
chmod +x /etc/init.d/webhook-relay
chmod +x /opt/webhook-relay/webhook-relay

# 添加到开机自启
rc-update add webhook-relay default

# 启动服务
rc-service webhook-relay start

# 查看状态
rc-service webhook-relay status
```

---

## Ubuntu/Debian 部署

### 一、系统准备

```bash
# 更新系统
apt update && apt upgrade -y

# 安装必要依赖
apt install -y go sqlite3 git
```

### 二、创建项目目录

```bash
mkdir -p /opt/webhook-relay
cd /opt/webhook-relay
```

### 三、编译项目

```bash
# 复制代码到 /opt/webhook-relay
# 然后编译
go build -o webhook-relay main.go
```

### 四、创建 Systemd 服务

```bash
cat > /etc/systemd/system/webhook-relay.service << 'EOF'
[Unit]
Description=Webhook Relay Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/webhook-relay
ExecStart=/opt/webhook-relay/webhook-relay
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
```

### 五、启动服务

```bash
# 重新加载 systemd
systemctl daemon-reload

# 启用开机自启
systemctl enable webhook-relay

# 启动服务
systemctl start webhook-relay

# 查看状态
systemctl status webhook-relay
```

---

## CentOS/RHEL 部署

### 一、系统准备

```bash
# 更新系统
yum update -y

# 安装必要依赖
yum install -y golang sqlite git
```

### 二、创建项目目录

```bash
mkdir -p /opt/webhook-relay
cd /opt/webhook-relay
```

### 三、编译项目

```bash
# 复制代码到 /opt/webhook-relay
go build -o webhook-relay main.go
```

### 四、创建 Systemd 服务

```bash
cat > /etc/systemd/system/webhook-relay.service << 'EOF'
[Unit]
Description=Webhook Relay Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/webhook-relay
ExecStart=/opt/webhook-relay/webhook-relay
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
```

### 五、启动服务

```bash
# 重新加载 systemd
systemctl daemon-reload

# 启用开机自启
systemctl enable webhook-relay

# 启动服务
systemctl start webhook-relay

# 查看状态
systemctl status webhook-relay
```

---

## Docker 部署

### 方式一：使用 Docker 运行

#### 1\. 创建数据目录

```bash
mkdir -p /opt/webhook-relay-data
```

#### 2\. 运行容器

```bash
docker run -d \
  --name webhook-relay \
  --restart always \
  -p 5000:5000 \
  -v /opt/webhook-relay-data:/data \
  golang:1.21 \
  sh -c "cd /opt/webhook-relay && go build -o webhook-relay main.go && ./webhook-relay"
```

#### 3\. 复制代码到容器并编译运行

```bash
# 先创建 Dockerfile
cat > Dockerfile << 'EOF'
FROM golang:1.21

WORKDIR /opt/webhook-relay

# 复制源代码
COPY . .

# 编译
RUN go build -o webhook-relay main.go

# 暴露端口
EXPOSE 5000

# 运行
CMD ["./webhook-relay"]
EOF
```

### 方式二：使用 Docker Compose

#### 1\. 创建 docker-compose.yml

```yaml
version: '3.8'

services:
  webhook-relay:
    build: .
    container_name: webhook-relay
    restart: always
    ports:
      - "5000:5000"
    volumes:
      - ./data:/data
    environment:
      - TZ=Asia/Shanghai
```

#### 2\. 构建并运行

```bash
# 构建镜像
docker-compose build

# 启动服务
docker-compose up -d

# 查看日志
docker-compose logs -f
```

### 方式三：使用预构建镜像

如果您有预构建的镜像：

```bash
docker run -d \
  --name webhook-relay \
  --restart always \
  -p 5000:5000 \
  -v /opt/webhook-relay-data:/data \
  your-registry/webhook-relay:latest
```

---

## 群晖 NAS 部署

### 方式一：使用 Docker

#### 1\. 安装 Docker 套件

在群晖「套件中心」安装「Docker」。

#### 2\. 创建文件夹

在 File Station 中创建：

- `/docker/webhook-relay` - 用于存放数据

#### 3\. 复制程序文件

将编译好的 `webhook-relay` 二进制文件和 `templates` 文件夹上传到 `/docker/webhook-relay`。

#### 4\. SSH 连接到群晖

```bash
ssh admin@your-nas-ip
sudo -i
```

#### 5\. 运行容器

```bash
docker run -d \
  --name webhook-relay \
  --restart always \
  -p 5000:5000 \
  -v /volume1/docker/webhook-relay:/data \
  golang:1.21 \
  sh -c "cd /opt/webhook-relay && ./webhook-relay"
```

### 方式二：使用任务计划（原生运行）

#### 1\. 安装 Go 环境

群晖使用 Intel/AMD 架构的可以尝试：

```bash
# 下载 Go
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz

# 添加到 PATH
export PATH=$PATH:/usr/local/go/bin
```

#### 2\. 复制并编译程序

将代码复制到群晖后：

```bash
cd /path/to/webhook-relay
go build -o webhook-relay main.go
```

#### 3\. 创建启动脚本

```bash
cat > /volume1/webhook-relay/start.sh << 'EOF'
#!/bin/bash
cd /volume1/webhook-relay
./webhook-relay
EOF
chmod +x /volume1/webhook-relay/start.sh
```

#### 4\. 在群晖控制面板创建任务计划

1. 打开「控制面板」&gt; 「任务计划」
2. 创建「触发的任务」&gt; 「用户定义的脚本」
3. 设置：
   - 任务名称：`webhook-relay`
   - 用户：`root`
   - 触发时间：「开机」
4. 添加脚本：`/volume1/webhook-relay/start.sh`

---

## 配置反向代理

### Nginx

```nginx
server {
    listen 80;
    server_name webhook.example.com;

    location / {
        proxy_pass http://127.0.0.1:5000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### 使用 HTTPS 反向代理

```nginx
server {
    listen 443 ssl;
    server_name webhook.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://127.0.0.1:5000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
    }
}
```

### Caddy

```caddy
webhook.example.com {
    reverse_proxy localhost:5000
}
```

---

## 防火墙配置

### iptables

```bash
# 允许 5000 端口
iptables -A INPUT -p tcp --dport 5000 -j ACCEPT

# 保存规则
service iptables save
```

### firewalld

```bash
# 允许 5000 端口
firewall-cmd --permanent --add-port=5000/tcp
firewall-cmd --reload
```

---

## 数据库管理

### 数据库限制

- 默认最大数据库大小：50MB
- 超过限制时自动清理 30 天前的数据
- 清理后执行 VACUUM 优化

### 手动备份

```bash
# 停止服务后备份
cp /opt/webhook-relay/webhook_relay.db /backup/webhook_relay.db.$(date +%Y%m%d)
```

### 手动恢复

```bash
# 停止服务
systemctl stop webhook-relay

# 恢复备份
cp /backup/webhook_relay.db.20240101 /opt/webhook-relay/webhook_relay.db

# 启动服务
systemctl start webhook-relay
```

---

## 常见问题

### Q: 服务无法启动？

检查日志：

```bash
# Alpine (OpenRC)
rc-service webhook-relay logs

# Debian/Ubuntu/CentOS (Systemd)
journalctl -u webhook-relay -f
```

### Q: 数据库权限问题？

```bash
chown -R root:root /opt/webhook-relay
chmod -R 755 /opt/webhook-relay
```

### Q: 如何查看服务状态？

```bash
# Systemd
systemctl status webhook-relay

# OpenRC
rc-service webhook-relay status

# Docker
docker ps | grep webhook-relay
```

### Q: 如何更新服务？

```bash
# 停止服务
systemctl stop webhook-relay

# 备份数据
cp -r /opt/webhook-relay/webhook_relay.db /backup/

# 重新编译
go build -o webhook-relay main.go

# 启动服务
systemctl start webhook-relay
```

### Q: HTTPS 代理后 Webhook 地址仍显示 HTTP？

确保反向代理配置了正确的头部：

```nginx
proxy_set_header X-Forwarded-Proto $scheme;
```

支持的代理头部：

- `X-Forwarded-Proto`
- `X-Forwarded-Protocol`
- `X-Url-Scheme`
- `X-Forwarded-Ssl`
- `Front-End-Https`

### Q: 忘记登录密码？

删除数据库后重新启动服务：

```bash
rm /opt/webhook-relay/webhook_relay.db
systemctl restart webhook-relay
```

这将重置所有配置，包括登录账号。

---

## 许可证

MIT License
