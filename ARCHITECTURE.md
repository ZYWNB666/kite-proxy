# kite-proxy 代码结构

本文档说明 kite-proxy 的代码组织和各个模块的职责。

## 目录结构

```
kite-proxy/
├── main.go                 # 应用入口点
├── go.mod                  # Go 模块定义
├── README.md               # 用户文档
├── VERIFICATION_PLAN.md    # 功能验证计划
│
├── pkg/                    # 可重用的包
│   └── api/                # Kite API 客户端
│       ├── client.go       # HTTP 客户端实现
│       ├── kubeconfig.go   # Kubeconfig 解析和构建
│       └── retry.go        # 重试机制和容错
│
├── server/                 # HTTP 服务器和业务逻辑
│   ├── server.go          # Gin 路由和服务器启动
│   ├── config.go          # 全局配置管理
│   ├── api_handlers.go    # REST API 处理器
│   ├── kubeconfig_cache.go # 内存中的 kubeconfig 缓存
│   ├── proxy.go           # K8s API 反向代理
│   ├── syncer.go          # 自动同步机制
│   └── dist/              # 嵌入的前端资源
│
└── ui/                     # Vue.js 前端 (独立构建)
    ├── src/
    ├── package.json
    └── vite.config.ts
```

## 模块说明

### `pkg/api` - Kite API 客户端

独立的、可测试的包，负责与 kite 服务器通信。

#### `client.go`

- **`Client`**: 基础 HTTP 客户端
  - `NewClient(kiteURL, apiKey)`: 创建客户端实例
  - `GetKubeconfigs(ctx, cluster)`: 获取 kubeconfig 列表
  - `GetClusterKubeconfig(ctx, cluster)`: 获取特定集群的 kubeconfig
  - `ListClusters(ctx)`: 获取集群名称列表
  - `Ping(ctx)`: 健康检查

- **数据结构**:
  - `ClusterKubeconfig`: 单个集群的 kubeconfig
  - `KubeconfigResponse`: API 响应结构

#### `kubeconfig.go`

- `ParseKubeconfig(yaml)`: 将 YAML 字符串解析为 `rest.Config`
- `BuildKubeconfigYAML(clusters, baseURL)`: 生成本地 kubeconfig 文件

#### `retry.go`

- **`RetryableClient`**: 带有重试逻辑的客户端包装器
  - 自动重试瞬态网络错误
  - 指数退避策略
  - 可配置的重试参数

- **`RetryConfig`**: 重试配置
  - `MaxRetries`: 最大重试次数（默认 3）
  - `InitialBackoff`: 初始退避时间（默认 500ms）
  - `MaxBackoff`: 最大退避时间（默认 10s）
  - `Multiplier`: 退避乘数（默认 2.0）

---

### `server/` - HTTP 服务器

#### `server.go`

应用的 HTTP 服务器，使用 Gin 框架。

- **`Server`**: 主服务器结构
  - `New(cfg)`: 创建服务器实例，初始化自动同步
  - `Run()`: 启动 HTTP 服务器

- **路由**:
  - `/ui/`: 静态前端资源（嵌入）
  - `/api/*`: REST API 端点
  - `/proxy/:cluster/*path`: K8s API 反向代理

#### `config.go`

全局配置管理，线程安全。

- **`Config`**: 运行时配置
  - `Port`: 监听端口
  - `KiteURL`: kite 服务器地址
  - `APIKey`: API 密钥（仅存于内存）

- **函数**:
  - `GetConfig()`: 获取当前配置（只读副本）
  - `SetConfig(cfg)`: 更新配置并清除缓存

#### `api_handlers.go`

REST API 端点的处理器函数。

| 处理器 | 端点 | 功能 |
|--------|------|------|
| `handleGetConfig` | `GET /api/config` | 返回配置（API key 已脱敏） |
| `handleSetConfig` | `POST /api/config` | 更新配置 |
| `handleListClusters` | `GET /api/clusters` | 列出可用集群 |
| `handleGetKubeconfig` | `GET /api/kubeconfig` | 生成本地 kubeconfig |
| `handleStatus` | `GET /api/status` | 返回状态和同步信息 |
| `handleTriggerSync` | `POST /api/sync` | 手动触发同步 |
| `handleClearCache` | `DELETE /api/cache` | 清除所有缓存 |
| `handlePrewarm` | `POST /api/cache/:cluster` | 预热特定集群 |

#### `kubeconfig_cache.go`

内存中的 kubeconfig 缓存，确保不写入磁盘。

- **`kubeconfigCache`**: 线程安全的缓存
  - `Get(cluster)`: 获取或加载 kubeconfig
  - `Clear()`: 清除所有缓存
  - `ClearCluster(cluster)`: 清除特定集群
  - `ListCached()`: 列出已缓存的集群

- **工作流程**:
  1. 首次访问集群 → 从 kite 获取 kubeconfig
  2. 解析 YAML → 转换为 `rest.Config`
  3. 丢弃原始 YAML，仅保留 `rest.Config`
  4. 后续访问复用缓存的 `rest.Config`

#### `proxy.go`

Kubernetes API 反向代理。

- **`HandleProxy`**: 主代理处理器
  - 从 URL 中提取集群名称
  - 加载该集群的 `rest.Config`
  - 构建认证的 HTTP round-tripper
  - 转发请求到真实的 K8s API 服务器
  - 返回响应给 kubectl

- **认证**: 使用 kubeconfig 中的凭证（证书、token 等）
- **TLS**: 由 `rest.Config` 处理 CA 验证

#### `syncer.go`

自动同步机制，定期检查 kite 服务器连接状态。

- **`Syncer`**: 同步器
  - `Start()`: 启动后台同步循环
  - `Stop()`: 停止同步
  - `SyncNow()`: 立即执行一次同步
  - `LastSyncError()`: 获取上次同步错误

- **全局同步器**:
  - `InitSyncer(interval)`: 初始化并启动（默认 5 分钟）
  - `StopSyncer()`: 停止全局同步器
  - `GetSyncStatus()`: 获取同步状态

- **工作流程**:
  1. 启动时延迟 2 秒后执行首次同步
  2. 每隔 `interval` 执行一次同步
  3. 使用 `Ping()` 验证 kite 服务器可达性
  4. 记录最后一次同步错误（如果有）

---

## 数据流

### 启动流程

```
main()
  └─> server.New(cfg)
        ├─> SetConfig(cfg)          # 设置全局配置
        └─> InitSyncer(5min)        # 启动自动同步
              └─> syncer.Start()
                    └─> [后台 goroutine] 每 5 分钟 Ping kite
  └─> server.Run()
        └─> gin.Run(port)           # 启动 HTTP 服务器
```

### kubectl 请求流程

```
kubectl get pods
  ↓
http://localhost:8090/proxy/my-cluster/api/v1/namespaces/default/pods
  ↓
HandleProxy(c)
  ├─> 提取集群名称: "my-cluster"
  ├─> globalCache.Get("my-cluster")
  │     ├─> [缓存命中] 返回 rest.Config
  │     └─> [缓存未命中]
  │           ├─> api.NewClient(kiteURL, apiKey)
  │           ├─> client.GetClusterKubeconfig(ctx, "my-cluster")
  │           │     ├─> GET https://kite.example.com/api/v1/proxy/kubeconfig?cluster=my-cluster
  │           │     └─> 返回 {"clusters":[{"name":"my-cluster","kubeconfig":"..."}]}
  │           ├─> api.ParseKubeconfig(yaml)  # 解析 YAML
  │           └─> 缓存 rest.Config，丢弃 YAML
  ├─> rest.TransportFor(restConfig)  # 构建认证的 HTTP 传输
  └─> httputil.ReverseProxy.ServeHTTP()  # 转发到 K8s API
        └─> 真实 K8s API 服务器
              └─> 返回 Pod 列表
```

### 配置更新流程

```
POST /api/config {"kiteURL":"...", "apiKey":"..."}
  ↓
handleSetConfig(c)
  ├─> SetConfig(newConfig)
  │     ├─> globalConfig = newConfig
  │     └─> globalCache.Clear()      # 清除所有缓存
  └─> 返回 200 OK
```

### 自动同步流程

```
[每 5 分钟]
  ↓
syncer.performSync()
  ├─> GetConfig()                    # 获取当前配置
  ├─> [如果未配置] 跳过
  └─> [如果已配置]
        ├─> api.NewClient(kiteURL, apiKey)
        ├─> client.Ping(ctx)
        │     └─> GET https://kite.example.com/api/v1/proxy/kubeconfig
        ├─> [成功] lastSyncErr = nil
        └─> [失败] lastSyncErr = err
```

---

## 安全考虑

### 内存安全

1. **API Key**: 
   - 仅存储在 `globalConfig` 中（进程内存）
   - 从不写入文件、日志或环境变量
   - 在 `/api/config` 响应中脱敏显示

2. **Kubeconfig**:
   - 原始 YAML 仅在 `ParseKubeconfig()` 中短暂存在
   - 立即解析为 `rest.Config` 并丢弃 YAML
   - `rest.Config` 包含证书、token 等敏感数据，但不是明文
   - 重启进程即清除所有缓存

3. **传输安全**:
   - kite-proxy → kite: 建议使用 HTTPS
   - kite-proxy → K8s API: 由 kubeconfig 中的 TLS 配置决定
   - kubectl → kite-proxy: 当前是 HTTP（生产环境建议加 TLS）

### RBAC 控制

- kite-proxy **不执行** RBAC 检查
- 所有权限控制由 kite 服务器和 K8s API 服务器处理
- kite 服务器在返回 kubeconfig 前检查 `allowProxy` 权限

---

## 测试和验证

参见 [VERIFICATION_PLAN.md](VERIFICATION_PLAN.md) 了解完整的验证流程。

### 快速测试

```bash
# 1. 启动 kite-proxy
./kite-proxy --port 8090 --kite-url http://localhost:8080 --api-key YOUR_KEY

# 2. 检查状态
curl http://localhost:8090/api/status

# 3. 列出集群
curl http://localhost:8090/api/clusters

# 4. 生成 kubeconfig
curl http://localhost:8090/api/kubeconfig -o test-kubeconfig.yaml

# 5. 使用 kubectl
export KUBECONFIG=test-kubeconfig.yaml
kubectl get pods -A
```

---

## 扩展点

如需扩展 kite-proxy，以下是常见的修改点：

1. **添加新的 API 端点**: 在 `server/api_handlers.go` 和 `server/server.go` 中添加
2. **修改缓存策略**: 编辑 `server/kubeconfig_cache.go`
3. **自定义重试逻辑**: 修改 `pkg/api/retry.go` 中的 `RetryConfig`
4. **添加认证中间件**: 在 `server/server.go` 的路由中添加 Gin 中间件
5. **集成监控指标**: 添加 Prometheus 中间件和指标导出器

---

## 依赖项

主要 Go 依赖：

- **gin-gonic/gin**: HTTP 框架
- **k8s.io/client-go**: Kubernetes 客户端库（用于解析 kubeconfig）
- **k8s.io/klog/v2**: 结构化日志

前端依赖：

- **Vue 3**: 前端框架
- **TypeScript**: 类型安全
- **Vite**: 构建工具

---

## 许可证

MIT License

---

## 贡献

欢迎提交 Issue 和 Pull Request！
