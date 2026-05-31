# 在 Alpine 3.23 系统上部署 Go 版本 Webhook Relay

本文档详细介绍如何在 Alpine 3.23 上部署该项目并设置开机自动运行。

## 方案一：交叉编译后部署 (推荐)

### 1. 交叉编译为 Linux/amd64

在 Windows 或其他系统上交叉编译：

```bash
# 编译为 Linux/amd64 (针对 Alpine 的 musl libc)
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "1"
go build -o webhook-relay-linux-amd64
```

或者在 Linux/Mac 上：
```bash
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o webhook-relay-linux-amd64
```

注意：如果目标系统是 musl libc (如 Alpine)，需要使用 musl 交叉编译工具链。

### 2. 上传文件到 Alpine 服务器

需要上传的文件：
- `webhook-relay-linux-amd64` (可执行文件)
- `templates/index.html` (Web 模板)

```bash
# 创建项目目录
mkdir -p /opt/webhook-relay
mkdir -p /opt/webhook-relay/templates

# 上传文件到服务器
scp webhook-relay-linux-amd64 user@server:/opt/webhook-relay/
scp templates/index.html user@server:/opt/webhook-relay/templates/

# 设置执行权限
chmod +x /opt/webhook-relay/webhook-relay-linux-amd64
```

### 3. 配置 OpenRC 服务 (Alpine 默认)

创建服务配置文件：

```bash
cat > /etc/init.d/webhook-relay << 'EOF'
#!/sbin/openrc-run

name="webhook-relay"
description="Webhook Relay Service"

command="/opt/webhook-relay/webhook-relay-linux-amd64"
command_background="yes"
command_user="nobody:nobody"

pidfile="/run/webhook-relay.pid"
directory="/opt/webhook-relay"

depend() {
    need net
    after firewall
}

start_pre() {
    checkpath -d -m 0755 -o nobody:nobody /opt/webhook-relay
}
EOF
```

赋予执行权限：
```bash
chmod +x /etc/init.d/webhook-relay
```

### 4. 添加到开机自启

```bash
# 添加到开机自启
rc-update add webhook-relay default

# 启动服务
rc-service webhook-relay start

# 查看服务状态
rc-service webhook-relay status
```

## 方案二：在 Alpine 上直接编译 (简单)

### 1. 安装必要依赖

```bash
# 更新系统
apk update && apk upgrade

# 安装 Go、Git 和编译工具
apk add go git musl-dev gcc
```

### 2. 下载项目

```bash
# 创建项目目录
mkdir -p /opt/webhook-relay
cd /opt/webhook-relay

# 如果有 Git 仓库，克隆项目
git clone <your-repo-url> .

# 或者直接上传项目文件到服务器
```

### 3. 编译项目

```bash
cd /opt/webhook-relay

# 下载依赖
go mod tidy

# 编译项目
go build -o webhook-relay
```

### 4. 创建必要目录和文件

```bash
# 确保模板目录存在
mkdir -p templates

# 如果 templates/index.html 没有上传，需要创建或上传
```

### 5. 设置服务 (OpenRC)

同方案一的步骤 3 和 4。

## 方案三：使用 Docker 部署 (可选)

创建 `Dockerfile`：

```dockerfile
FROM alpine:3.23

# 安装必要包
RUN apk add --no-cache ca-certificates

# 创建工作目录
WORKDIR /app

# 复制项目文件
COPY webhook-relay /app/
COPY templates/ /app/templates/

# 暴露端口
EXPOSE 5000

# 启动服务
CMD ["/app/webhook-relay"]
```

构建和运行：

```bash
# 构建镜像
docker build -t webhook-relay .

# 运行容器
docker run -d \
  --name webhook-relay \
  -p 5000:5000 \
  -v /opt/webhook-relay/webhook_relay.db:/app/webhook_relay.db \
  --restart unless-stopped \
  webhook-relay
```

## 防火墙配置

如果需要从外部访问，开放 5000 端口：

```bash
# 使用 iptables
iptables -A INPUT -p tcp --dport 5000 -j ACCEPT
iptables-save > /etc/iptables/rules-save

# 或使用 ufw (如果已安装)
ufw allow 5000/tcp
```

## 验证部署

```bash
# 检查服务状态
rc-service webhook-relay status

# 检查日志
tail -f /var/log/messages

# 测试服务是否运行
curl http://localhost:5000/status

# 访问 Web 界面
# 浏览器打开 http://<server-ip>:5000
```

## 常见问题

### 1. 依赖问题

如果遇到依赖问题，确保安装：
```bash
apk add --no-cache ca-certificates musl-dev
```

### 2. 权限问题

确保运行用户对项目目录有读写权限：
```bash
chown -R nobody:nobody /opt/webhook-relay
chmod -R 755 /opt/webhook-relay
```

### 3. 数据库文件权限

SQLite 数据库文件需要写入权限：
```bash
touch /opt/webhook-relay/webhook_relay.db
chown nobody:nobody /opt/webhook-relay/webhook_relay.db
chmod 644 /opt/webhook-relay/webhook_relay.db
```

## 管理命令

```bash
# 启动服务
rc-service webhook-relay start

# 停止服务
rc-service webhook-relay stop

# 重启服务
rc-service webhook-relay restart

# 查看状态
rc-service webhook-relay status

# 查看日志
tail -f /var/log/messages
```

## 数据持久化

重要文件：
- `/opt/webhook-relay/webhook_relay.db` - SQLite 数据库，包含规则和端点配置
- `/opt/webhook-relay/templates/index.html` - Web 界面模板

建议定期备份数据库文件。
