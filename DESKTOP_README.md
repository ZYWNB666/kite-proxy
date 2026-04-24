# Kite Proxy 桌面应用版本

## 功能说明

桌面应用版本提供了以下功能：

### 主要特性

- **本地应用**：无需浏览器，双击即可运行
- **系统原生**：使用 Wails 框架，性能优秀
- **内存安全**：所有数据仅存于应用内存中
- **跨平台**：支持 Windows、macOS、Linux

### 与 Web 版本的区别

| 特性 | Web 版本 | 桌面版本 |
|------|---------|---------|
| 运行方式 | 需要浏览器访问 | 双击可执行文件 |
| 端口占用 | 需要 8090 端口 | 不占用端口 |
| 自动同步 | 后台5分钟同步 | 手动测试连接 |
| 数据持久化 | 重启丢失 | 重启丢失 |
| 系统通知 | 不支持 | 支持原生通知 |

## 快速开始

### 1. 构建桌面应用

```powershell
# 一键构建
.\build-desktop.ps1
```

构建完成后会生成：`build\bin\kite-proxy.exe`

### 2. 运行

直接双击 `build\bin\kite-proxy.exe` 即可运行！

### 3. 配置

首次运行时：
1. 在 **Configuration** 页面输入 kite 服务器地址
2. 输入 API Key
3. 点击 **Test Connection** 测试连接
4. 点击 **Save** 保存配置

### 4. 使用

- **Clusters** 页面：查看可访问的集群，预热缓存
- **Usage** 页面：下载 kubeconfig，查看使用说明

## 开发模式

如需开发调试，使用热重载模式：

```powershell
.\dev-desktop.ps1
```

这会启动开发服务器，前端修改会自动刷新。

## 技术栈

- **后端**：Go 1.25 + Wails v2
- **前端**：Vue 3 + TypeScript + Vite
- **UI**：Element Plus
- **打包**：Wails Build

## 构建选项

### 开发构建（带调试）

```powershell
wails build -f main_desktop.go -debug
```

### 生产构建（优化）

```powershell
wails build -f main_desktop.go -clean -trimpath -s
```

### 跨平台构建

```powershell
# Linux
wails build -f main_desktop.go -platform linux/amd64

# macOS
wails build -f main_desktop.go -platform darwin/universal

# Windows
wails build -f main_desktop.go -platform windows/amd64
```

## 文件说明

```
kite-proxy/
├── desktop/            # 桌面应用后端
│   ├── app.go         # 主应用逻辑
│   └── cache.go       # 缓存管理
├── main_desktop.go    # 桌面版入口
├── wails.json         # Wails 配置
├── build-desktop.ps1  # 构建脚本
├── dev-desktop.ps1    # 开发脚本
└── ui/
    └── src/
        └── api/
            └── adapter.ts  # API 适配器（Web/Desktop）
```

## 常见问题

### Q: 构建失败 "wails: command not found"

A: 运行 `.\build-desktop.ps1` 会自动安装 Wails。或手动安装：

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
$env:PATH += ";$env:GOPATH\bin"
```

### Q: 前端构建失败

A: 确保已安装 Node.js，然后：

```powershell
cd ui
npm install
npm run build
```

### Q: 运行时报错 "无法连接到 kite 服务器"

A: 确保：
1. kite 服务器正在运行
2. URL 正确（如 http://localhost:8080）
3. API Key 有效且有 `allowProxy: true` 权限

### Q: 如何更新配置

A: 在应用的 **Configuration** 页面修改并保存即可。

### Q: 桌面版是否支持多集群

A: 是的，与 Web 版本功能一致，支持多集群管理。

### Q: 能否同时运行 Web 版和桌面版

A: 可以，它们是独立的应用，不会互相干扰。

## 打包发布

### Windows 安装包

使用 NSIS 或 Inno Setup 打包：

```powershell
# 使用 Wails 自带的打包
wails build -f main_desktop.go -nsis
```

### 便携版

直接分发 `build\bin\kite-proxy.exe` 即可，无需安装。

## 安全说明

### 数据存储

- **API Key**：仅存于应用运行时内存
- **Kubeconfig**：解析后仅保留 rest.Config 对象
- **关闭应用**：所有数据自动清除

### 网络通信

- 仅与配置的 kite 服务器通信
- 不收集任何使用数据
- 不连接外部服务

### 建议

- 使用 HTTPS 连接 kite 服务器
- 定期更新 API Key
- 不要在不可信环境运行

## 性能优化

### 减小体积

编译时使用优化选项：

```powershell
wails build -f main_desktop.go -clean -trimpath -s -ldflags "-s -w"
```

### 加快启动

- 预热常用集群
- 使用缓存功能

## 更新日志

### v1.0.0 (2026-04-24)

- 初始版本
- 支持基本的集群管理
- kubeconfig 获取和缓存
- 配置管理
- 系统通知支持

## 路线图

- [ ] 系统托盘集成
- [ ] 自动启动选项
- [ ] 配置导入/导出
- [ ] 多语言支持
- [ ] 主题定制

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

MIT License
