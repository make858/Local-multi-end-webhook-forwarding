@echo off
REM Webhook Relay 交叉编译脚本 - Windows -> Linux
REM 适用于 Windows 上编译 Linux/amd64 版本

echo ========================================
echo Webhook Relay 交叉编译工具
echo ========================================
echo.

REM 设置环境变量
set GOOS=linux
set GOARCH=amd64

REM 尝试启用 CGO (用于 SQLite)
set CGO_ENABLED=1

echo 正在编译 Linux/amd64 版本...
echo 环境变量:
echo   GOOS=%GOOS%
echo   GOARCH=%GOARCH%
echo   CGO_ENABLED=%CGO_ENABLED%
echo.

REM 执行编译
go build -o webhook-relay-linux-amd64

if %ERRORLEVEL% EQU 0 (
    echo.
    echo ========================================
    echo 编译成功！
    echo 输出文件: webhook-relay-linux-amd64
    echo ========================================
    echo.
    echo 部署步骤:
    echo 1. 上传以下文件到 Alpine 服务器:
    echo    - webhook-relay-linux-amd64
    echo    - templates/index.html
    echo 2. 在服务器上运行:
    echo    chmod +x deploy-alpine.sh
    echo    ./deploy-alpine.sh
    echo.
) else (
    echo.
    echo ========================================
    echo 编译失败！
    echo ========================================
    echo.
    echo 如果是 CGO 问题，请尝试以下方案:
    echo 方案 1: 在 Alpine 服务器上直接编译 (推荐)
    echo 方案 2: 使用 Docker 编译
    echo.
)

pause
