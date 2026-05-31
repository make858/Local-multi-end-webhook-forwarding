@echo off
echo ========================================
echo   Webhook Relay 一键部署工具
echo ========================================
echo.

cd /d "%~dp0deploy"

if not exist "go.mod" (
    echo 初始化模块...
    go mod init deploy
    go mod tidy
)

if not exist "deploy.exe" (
    echo 正在编译部署工具...
    go build -o deploy.exe main.go
    if errorlevel 1 (
        echo 编译失败！
        pause
        exit /b 1
    )
)

echo 正在部署...
deploy.exe

echo.
pause
