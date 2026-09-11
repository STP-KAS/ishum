Set-Location $PSScriptRoot
if (-not (Test-Path .\ishum.exe)) {
  go build -o ishum.exe ./cmd/ishum
}
Get-Process ishum -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Process -FilePath "$PSScriptRoot\ishum.exe" -WorkingDirectory $PSScriptRoot -WindowStyle Minimized
Start-Sleep 1
Write-Output "http://127.0.0.1:8090/pos"
