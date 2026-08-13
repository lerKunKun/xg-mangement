$env:DATABASE_URL = "postgres://xg:xg@localhost:5433/xg?sslmode=disable"
$env:AUTH_DEV_LOGIN_ENABLED = "true"
$env:CREDENTIAL_ENCRYPTION_KEY = "0123456789abcdef0123456789abcdef"
$env:WEB_BASE_URL = "http://localhost:3001"
$env:REDIS_URL = "redis://localhost:6379/0"
$env:OBJECT_STORAGE_ENDPOINT = "http://localhost:9000"

Start-Process -FilePath "go" -ArgumentList "run", "./cmd/api" -WorkingDirectory "$PSScriptRoot/../backend" -WindowStyle Hidden -RedirectStandardOutput "$PSScriptRoot/../.api.out.log" -RedirectStandardError "$PSScriptRoot/../.api.err.log" -PassThru
