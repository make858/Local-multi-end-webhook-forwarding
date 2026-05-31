$secpasswd = ConvertTo-SecureString "123456" -AsPlainText -Force
$credential = New-Object System.Management.Automation.PSCredential ("root", $secpasswd)

$sessionOptions = New-Object Microsoft.Win32.OpenSSH.Server.SessionOptions
$sessionOptions.Port = 22

Write-Host "Connecting to server..."
$session = New-SSHSession -ComputerName "10.10.0.139" -Credential $credential -AcceptKey

if ($session) {
    Write-Host "Connected successfully!"
    
    Write-Host "Backing up existing template..."
    Invoke-SSHCommand -SessionId $session.SessionId -Command "cd /opt/webhook-relay && mv templates/index.html templates/index.html.bak 2>/dev/null || true"
    
    Write-Host "Transferring new template..."
    Set-SCPFile -ComputerName "10.10.0.139" -Credential $credential -LocalFile "e:\pm\webhook-relay\templates\index.html" -RemotePath "/opt/webhook-relay/templates/index.html" -AcceptKey
    
    Write-Host "Restarting service..."
    Invoke-SSHCommand -SessionId $session.SessionId -Command "rc-service webhook-relay restart"
    
    Remove-SSHSession -SessionId $session.SessionId
    Write-Host "Done!"
} else {
    Write-Host "Failed to connect to server. Trying alternative method..."
    Write-Host "Please manually copy the following file to the server:"
    Write-Host "  Source: e:\pm\webhook-relay\templates\index.html"
    Write-Host "  Target: /opt/webhook-relay/templates/index.html"
    Write-Host ""
    Write-Host "Then run on the server:"
    Write-Host "  rc-service webhook-relay restart"
}
