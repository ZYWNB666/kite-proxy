# Kite Proxy 桌面应用 - 快速开始

## 🎉 构建成功！

桌面应用已生成：**`build\bin\kite-proxy.exe`** (约 15 MB)

## 🚀 运行方式

### 方式一：直接运行（推荐）

双击 `build\bin\kite-proxy.exe` 即可启动！

### 方式二：命令行运行

```powershell
.\build\bin\kite-proxy.exe
```

## 📝 首次使用

1. **启动应用**
   - 双击 `kite-proxy.exe`
   - 应用窗口会自动打开（1200x800）

2. **配置连接**
   - 点击 **Configuration** 标签
   - 输入 **Kite Server URL**（如 `http://localhost:8080`）
   - 输入 **API Key**
   - 点击 **Test Connection** 测试连接
   - 点击 **Save** 保存配置

3. **查看集群**
   - 点击 **Clusters** 标签
   - 查看所有可访问的 K8s 集群
   - 点击 **Prewarm** 预加载集群配置到缓存

4. **下载 Kubeconfig**
   - 点击 **Usage** 标签
   - 点击 **Download kubeconfig** 下载配置文件
   - 保存到 `~/.kube/config`

5. **使用 kubectl**
   ```bash
   kubectl --kubeconfig=./config get pods
   ```

## 🔧 开发模式

如需修改代码，使用开发模式：

```powershell
# 修改 Go 代码后
cd f:\kite-proxy
$env:PATH += ";$env:GOPATH\bin"
wails dev -tags desktop
```

前端会自动热重载，Go 代码需要重启。

## 🔨 重新构建

修改代码后重新构建：

```powershell
# 方式一：使用构建脚本
.\build-desktop.ps1

# 方式二：手动构建
cd ui
npm run build
cd ..
wails build -tags desktop -skipbindings -s
```

## 📁 文件结构

```
kite-proxy/
├── build/
│   └── bin/
│       └── kite-proxy.exe    # 🎯 可执行文件
├── desktop/
│   ├── app.go                # 桌面应用后端
│   └── cache.go              # 缓存管理
├── main_desktop.go           # 桌面版入口
├── ui/
│   ├── dist/                 # 前端构建产物（已嵌入）
│   └── src/
│       └── api/
│           └── adapter.ts    # Web/Desktop 适配器
└── wails.json                # Wails 配置
```

## 🌟 功能特性

### 已实现

- ✅ **原生桌面应用** - 无需浏览器
- ✅ **配置管理** - 内存存储，安全可靠
- ✅ **集群列表** - 查看所有可访问集群
- ✅ **缓存预热** - 加速 kubectl 访问
- ✅ **Kubeconfig 生成** - 一键下载配置
- ✅ **连接测试** - 验证 kite 服务器可用性
- ✅ **自适应 UI** - 自动检测 Web/Desktop 环境

### 与 Web 版本的区别

| 功能 | Web 版本 | 桌面版本 |
|------|---------|---------|
| 运行方式 | 浏览器访问 :8090 | 双击 exe |
| 端口占用 | 占用 8090 | 不占用端口 |
| 配置持久化 | 仅内存 | 仅内存 |
| 自动同步 | 5分钟同步 | 手动测试 |
| 系统通知 | 不支持 | 支持（待实现）|
| 安装 | 无需安装 | 无需安装 |

## ⚠️ 已知限制

### 当前版本限制

1. **配置不持久化**
   - 关闭应用后配置会丢失
   - 每次启动需要重新输入配置
   - 未来版本将支持配置文件

2. **代理功能未实现**
   - 当前仅支持 kubeconfig 下载
   - kubectl 代理功能需要同时运行 Web 版本
   - 未来将集成完整代理功能

3. **缓存不持久化**
   - 重启应用缓存会清空
   - 这是安全设计，避免泄露凭证

### 解决方案

如需使用 kubectl 代理功能，同时运行 Web 版本：

```powershell
# 终端 1：运行桌面应用
.\build\bin\kite-proxy.exe

# 终端 2：运行 Web 服务器（提供代理功能）
go run main.go -kite-url http://your-kite-server -api-key your-key
```

然后 kubeconfig 中的 `server` 指向 `http://localhost:8090/proxy/{cluster}`

## 🐛 故障排除

### Q: 双击后无反应

A: 检查：
1. 是否安装了 WebView2（Windows 10+ 通常已内置）
2. 查看日志：应用会输出到控制台
3. 尝试在 PowerShell 中运行查看错误

### Q: 无法连接到 kite 服务器

A: 确保：
1. kite 服务器正在运行
2. URL 格式正确（如 `http://localhost:8080`）
3. API Key 有效且有 `allowProxy: true` 权限
4. 防火墙未阻止连接

### Q: 集群列表为空

A: 检查：
1. API Key 是否有权限访问集群
2. kite 服务器配置是否正确
3. 点击 **Test Connection** 查看错误信息

### Q: 下载的 kubeconfig 无法使用

A: 注意：
1. 桌面版生成的 kubeconfig 需要 Web 服务器配合
2. 或者直接使用 kite 服务器的 kubeconfig
3. 将来版本会集成完整代理功能

## 🔐 安全说明

### 数据存储

- ✅ API Key 仅存于内存
- ✅ 关闭应用自动清除所有数据
- ✅ 不写入磁盘，不保存日志
- ⚠️ 每次启动需重新配置

### 网络通信

- ✅ 仅与配置的 kite 服务器通信
- ✅ 不收集任何使用数据
- ✅ 不连接外部服务
- 💡 建议使用 HTTPS 连接 kite 服务器

## 📦 分发部署

### 单文件分发

```powershell
# 直接复制 exe 文件即可
copy build\bin\kite-proxy.exe \\target-machine\
```

### 创建安装包（可选）

```powershell
# 需要安装 NSIS
wails build -tags desktop -nsis
```

### 便携版

`kite-proxy.exe` 本身就是便携版，无需安装！

## 🚀 性能优化

### 减小文件大小

```powershell
# 使用压缩和裁剪
wails build -tags desktop -skipbindings -s -clean -trimpath -upx -ldflags "-s -w"
```

### 加快启动速度

1. 预热常用集群
2. 使用缓存功能
3. 避免频繁重启

## 📚 相关文档

- [API 参考](API_REFERENCE.md)
- [架构设计](ARCHITECTURE.md)
- [Web 版本快速开始](QUICKSTART.md)
- [完整 README](README.md)

## 💡 提示与技巧

### 快捷键（计划中）

- `Ctrl+R` - 刷新集群列表
- `Ctrl+S` - 保存配置
- `Ctrl+Q` - 退出应用

### 命令行参数（计划中）

```powershell
# 调试模式
.\kite-proxy.exe -debug

# 指定配置文件
.\kite-proxy.exe -config config.yaml
```

## 🛠️ 开发信息

### 技术栈

- **后端**: Go 1.25 + Wails v2.12.0
- **前端**: React 19 + TypeScript + Tailwind CSS
- **打包**: Wails Build System
- **渲染**: WebView2 (Windows)

### 构建时间

- 前端构建：~3-4 秒
- Go 编译：~8-9 秒
- 总计：~12-13 秒

### 依赖版本

```
wails: v2.12.0
go: 1.25+
node: 24.11+
npm: 11.6+
webview2: 已内置（Windows 10+）
```

## 📝 更新日志

### v1.0.0 (2026-04-24)

- ✅ 初始发布
- ✅ 基本配置管理
- ✅ 集群列表查看
- ✅ Kubeconfig 生成
- ✅ 缓存管理
- ✅ 连接测试

### 计划功能

- [ ] 配置文件持久化
- [ ] 系统托盘集成
- [ ] 自动启动选项
- [ ] 内置代理服务器
- [ ] 多语言支持
- [ ] 主题定制
- [ ] 快捷键支持
- [ ] 系统通知

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 许可证

MIT License

---

**享受使用 Kite Proxy 桌面应用！** 🎉
