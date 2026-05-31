@echo off
echo Deploying template to server...
echo Using pscp to transfer file...
echo 123456 | "C:\Program Files\PuTTY\pscp.exe" -pw 123456 -scp e:\pm\webhook-relay\templates\index.html root@10.10.0.139:/opt/webhook-relay/templates/
if %errorlevel% neq 0 (
    echo Error: pscp not found or transfer failed
    echo Trying alternative method...
    echo Please manually copy templates/index.html to /opt/webhook-relay/templates/ on the server
) else (
    echo Template deployed successfully!
    echo Restarting service...
    "C:\Program Files\PuTTY\plink.exe" -pw 123456 root@10.10.0.139 "rc-service webhook-relay restart"
)
pause
