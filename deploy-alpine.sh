#!/bin/bash
# Webhook Relay 部署脚本 - Alpine 3.23
# 使用方法: ./deploy-alpine.sh

set -e

echo "========================================"
echo "Webhook Relay 部署工具 (Alpine 3.23)"
echo "========================================"

# 配置项
PROJECT_DIR="/opt/webhook-relay"
SERVICE_NAME="webhook-relay"
USER="nobody"
GROUP="nobody"

# 检查是否在 Alpine 系统上
if [ ! -f /etc/alpine-release ]; then
    echo "错误: 此脚本仅适用于 Alpine Linux 系统"
    exit 1
fi

# 创建项目目录
echo "步骤 1: 创建项目目录..."
mkdir -p $PROJECT_DIR
mkdir -p $PROJECT_DIR/templates

# 检查必要文件是否存在
echo "步骤 2: 检查文件..."
if [ ! -f "templates/index.html" ]; then
    echo "警告: templates/index.html 不存在，请确保已上传此文件"
fi

# 检查编译的可执行文件
if [ ! -f "main.go" ] && [ ! -f "webhook-relay" ]; then
    echo "错误: 找不到 main.go 或 webhook-relay 可执行文件"
    exit 1
fi

# 安装 Go 和编译工具（如果需要）
if [ -f "main.go" ] && ! command -v go >/dev/null 2>&1; then
    echo "步骤 3: 安装 Go 和编译工具..."
    apk add --no-cache go git musl-dev gcc
fi

# 编译项目（如果需要）
if [ -f "main.go" ] && [ ! -f "webhook-relay" ]; then
    echo "步骤 4: 编译项目..."
    go mod tidy
    go build -o webhook-relay
fi

# 复制文件到目标位置
echo "步骤 5: 复制文件..."
if [ -f "webhook-relay" ]; then
    cp webhook-relay $PROJECT_DIR/
elif [ -f "webhook-relay-linux-amd64" ]; then
    cp webhook-relay-linux-amd64 $PROJECT_DIR/webhook-relay
fi

if [ -f "templates/index.html" ]; then
    cp templates/index.html $PROJECT_DIR/templates/
fi

# 设置权限
echo "步骤 6: 设置权限..."
chmod +x $PROJECT_DIR/webhook-relay
chown -R $USER:$GROUP $PROJECT_DIR
chmod -R 755 $PROJECT_DIR

# 创建服务文件
echo "步骤 7: 创建 OpenRC 服务..."
cat > /etc/init.d/$SERVICE_NAME << 'EOF'
#!/sbin/openrc-run

name="webhook-relay"
description="Webhook Relay Service"

command="/opt/webhook-relay/webhook-relay"
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

chmod +x /etc/init.d/$SERVICE_NAME

# 添加到开机自启
echo "步骤 8: 添加到开机自启..."
rc-update add $SERVICE_NAME default

# 启动服务
echo "步骤 9: 启动服务..."
rc-service $SERVICE_NAME start

echo ""
echo "========================================"
echo "部署完成！"
echo "========================================"
echo "服务状态:"
rc-service $SERVICE_NAME status
echo ""
echo "访问地址:"
echo "http://localhost:5000"
echo ""
echo "管理命令:"
echo "启动: rc-service $SERVICE_NAME start"
echo "停止: rc-service $SERVICE_NAME stop"
echo "重启: rc-service $SERVICE_NAME restart"
echo "状态: rc-service $SERVICE_NAME status"
echo ""
echo "日志查看:"
echo "tail -f /var/log/messages"
echo "========================================"
