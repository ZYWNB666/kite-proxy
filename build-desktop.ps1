# Build kite-proxy desktop app
Write-Host "====================================" -ForegroundColor Cyan
Write-Host "Build Kite Proxy Desktop App" -ForegroundColor Cyan
Write-Host "====================================" -ForegroundColor Cyan
Write-Host ""

# 1. Check Wails
Write-Host "1. Checking Wails..." -ForegroundColor Yellow
$wailsCmd = Get-Command wails -ErrorAction SilentlyContinue
if (-not $wailsCmd) {
    $env:PATH += ";$env:GOPATH\bin"
    $wailsCmd = Get-Command wails -ErrorAction SilentlyContinue
}
if (-not $wailsCmd) {
    Write-Host "   Wails not found, installing..." -ForegroundColor Yellow
    go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
    $env:PATH += ";$env:GOPATH\bin"
} else {
    Write-Host "   Wails is installed." -ForegroundColor Green
}
Write-Host ""

# 2. Check Node.js
Write-Host "2. Checking Node.js..." -ForegroundColor Yellow
$nodeCmd = Get-Command node -ErrorAction SilentlyContinue
if (-not $nodeCmd) {
    Write-Host "   Node.js not found. Please install Node.js first." -ForegroundColor Red
    exit 1
}
Write-Host "   Node.js is installed." -ForegroundColor Green
Write-Host ""

# 3. Build frontend
Write-Host "3. Building frontend..." -ForegroundColor Yellow
Push-Location ui
if (-not (Test-Path "node_modules")) {
    Write-Host "   Installing npm dependencies..." -ForegroundColor Gray
    npm install
}
npm run build
Pop-Location
Write-Host "   Frontend build complete." -ForegroundColor Green
Write-Host ""

# 4. Build desktop app with Wails
Write-Host "4. Building desktop app..." -ForegroundColor Yellow
Write-Host "   This may take a few minutes..." -ForegroundColor Gray
wails build -tags desktop -skipbindings -s

if ($LASTEXITCODE -eq 0) {
    Write-Host ""
    Write-Host "====================================" -ForegroundColor Green
    Write-Host "Build succeeded!" -ForegroundColor Green
    Write-Host "====================================" -ForegroundColor Green
    Write-Host ""
    Write-Host "Output: build\bin\kite-proxy.exe" -ForegroundColor Cyan
} else {
    Write-Host ""
    Write-Host "====================================" -ForegroundColor Red
    Write-Host "Build failed." -ForegroundColor Red
    Write-Host "====================================" -ForegroundColor Red
    exit 1
}
