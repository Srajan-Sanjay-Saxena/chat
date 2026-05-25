# PowerShell / Windows — Process Management (compact)

Core commands
- `Get-Process`, `Stop-Process`, `Start-Process`, `Get-NetTCPConnection`, `tasklist`, `taskkill`.

Common examples
- List processes: `Get-Process`
- Kill by PID: `Stop-Process -Id 5080 -Force` or `taskkill /PID 5080 /F`
- Launch: `Start-Process -FilePath "C:\path\to\app.exe" -ArgumentList "-arg1 val"`
- Wait: `Wait-Process -Id 5080`

Notes
- Use `Get-NetTCPConnection -LocalPort 8080` to map ports to processes.
- Use `-ErrorAction` to silence non-fatal failures in scripts.

Sources
- Microsoft docs: https://learn.microsoft.com/powershell/