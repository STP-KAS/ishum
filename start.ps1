Set-Location $PSScriptRoot
if (-not (Test-Path .\ishum.exe)) {
  go build -o ishum.exe ./cmd/ishum
}
Get-Process ishum -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Milliseconds 400
# Win32_Process.Create leaves the parent job. Start-Process children die when
# the console/job that launched them exits (CTRL_CLOSE / job teardown).
$r = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{
  CommandLine      = "`"$PSScriptRoot\ishum.exe`""
  CurrentDirectory = "$PSScriptRoot"
}
if ($r.ReturnValue -ne 0) { throw "Ishum start failed: $($r.ReturnValue)" }
Start-Sleep 1
Write-Output "http://127.0.0.1:8090/pos"
