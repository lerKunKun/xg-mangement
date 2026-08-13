$env:API_INTERNAL_URL = "http://127.0.0.1:8080"
Start-Process -FilePath "npm.cmd" -ArgumentList "run", "dev", "--", "-p", "3001" -WorkingDirectory "$PSScriptRoot/../apps/web" -WindowStyle Hidden -RedirectStandardOutput "$PSScriptRoot/../.web.out.log" -RedirectStandardError "$PSScriptRoot/../.web.err.log" -PassThru
