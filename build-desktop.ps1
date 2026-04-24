# 构建 kite-proxy 桌面应用

Write-Host "====================================" -ForegroundColor Cyan
Write-Host "构建 Kite Proxy 桌面应用" -ForegroundColor Cyan
Write-Host "====================================" -ForegroundColor Cyan
Write-Host ""

# 检查 Wails 是否已安装
Write-Host "1. 检查 Wails..." -ForegroundColor Yellow
$wailsCmd = Get-Command wails -ErrorAction SilentlyContinue
if (-not $wailsCmd) {
    # 添加 GOPATH/bin 到 PATH
    $env:PATH += ";$env:GOPATH\bin"
    $wailsCmd = Get-Command wails -ErrorAction SilentlyContinue
}

if (-not $wailsCmd) {
    Write-Host "   ✗ Wails 未安装" -ForegroundColor Red
    Write-Host "   正在安装 Wails..." -ForegroundColor Yellow
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
    $env:PATH += ";$env:GOPATH\bin"
} else {
    Write-Host "   ✓ Wails 已安装: $($wailsCmd.Version)" -ForegroundColor Green
}
Write-Host ""

# 检查 Node.js
Write-Host "2. 检查 Node.js..." -ForegroundColor Yellow
$nodeCmd = Get-Command node -ErrorAction SilentlyContinue
if (-not $nodeCmd) {
    Write-Host "   ✗ Node.js 未安装，请先安装 Node.js" -ForegroundColor Red
    exit 1
}
Write-Host "   ✓ Node.js 已安装" -ForegroundColor Green
Write-Host ""

# 构建前端
Write-Host "3. 构建前端..." -ForegroundColor Yellow
cd ui
if (-not (Test-Path "node_modules")) {
    Write-Host "   安装依赖..." -ForegroundColor Gray
    npm install
}
Write-Host "   编译前端..." -ForegroundColor Gray
npm run build
cd ..
Write-Host "   ✓ 前端构建完成" -ForegroundColor Green
Write-Host ""

# 使用 Wails 构建桌面应用
Write-Host "4. 构建桌面应用..." -ForegroundColor Yellow
Write-Host "   这可能需要几分钟..." -ForegroundColor Gray
wails build -f main_desktop.go

if ($LASTEXITCODE -eq 0) {
    Write-Host ""
    Write-Host "====================================" -ForegroundColor Green
    Write-Host "✓ 构建成功！" -ForegroundColor Green
    Write-Host "====================================" -ForegroundColor Green
    Write-Host ""
    Write-Host "可执行文件位置：" -ForegroundColor Cyan
    Write-Host "  build\bin\kite-proxy.exe" -ForegroundColor White
    Write-Host ""
    Write-Host "运行方式：" -ForegroundColor Cyan
    Write-Host "  双击运行：build\bin\kite-proxy.exe" -ForegroundColor White
    Write-Host "  或命令行：.\build\bin\kite-proxy.exe" -ForegroundColor White
    Write-Host ""
} else {
    Write-Host ""
    Write-Host "====================================" -ForegroundColor Red
    Write-Host "✗ 构建失败" -ForegroundColor Red
    Write-Host "====================================" -ForegroundColor Red
    Write-Host ""
    Write-Host "请检查错误信息并重试" -ForegroundColor Yellow
    exit 1
}
