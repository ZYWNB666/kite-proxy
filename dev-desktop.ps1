# 开发模式运行 kite-proxy 桌面应用（带热重载）

Write-Host "====================================" -ForegroundColor Cyan
Write-Host "启动 Kite Proxy 桌面应用（开发模式）" -ForegroundColor Cyan
Write-Host "====================================" -ForegroundColor Cyan
Write-Host ""

# 确保 PATH 包含 GOPATH/bin
$env:PATH += ";$env:GOPATH\bin"

# 检查 Wails
$wailsCmd = Get-Command wails -ErrorAction SilentlyContinue
if (-not $wailsCmd) {
    Write-Host "✗ Wails 未安装" -ForegroundColor Red
    Write-Host "请先运行：.\build-desktop.ps1" -ForegroundColor Yellow
    exit 1
}

Write-Host "启动开发服务器..." -ForegroundColor Yellow
Write-Host "- 前端自动热重载" -ForegroundColor Gray
Write-Host "- 修改 Go 代码需要重启" -ForegroundColor Gray
Write-Host ""
Write-Host "按 Ctrl+C 停止" -ForegroundColor Yellow
Write-Host ""

# 运行开发模式
wails dev -f main_desktop.go
