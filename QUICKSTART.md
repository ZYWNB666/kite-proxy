# kite-proxy 快速开始指南

本指南帮助你在 5 分钟内启动 kite-proxy 并使用 kubectl 访问远程 Kubernetes 集群。

## 前置条件

1. **kite 服务器**运行中（例如：`http://localhost:8080`）
2. 拥有 **API Key**，且对应角色有 `allowProxy: true` 权限
3. 安装了 **kubectl** 命令行工具

## 步骤 1：构建 kite-proxy

```powershell
# 克隆或进入项目目录
cd kite-proxy

# 构建前端
cd ui
npm install
npm run build
cd ..

# 构建 Go 应用
go build -o kite-proxy.exe .
```

## 步骤 2：启动 kite-proxy

### 方式 A：命令行参数

```powershell
.\kite-proxy.exe `
  --port 8090 `
  --kite-url http://localhost:8080 `
  --api-key kite123-your-api-key-here
```

### 方式 B：环境变量

```powershell
$env:PORT = "8090"
$env:KITE_URL = "http://localhost:8080"
$env:KITE_API_KEY = "kite123-your-api-key-here"
.\kite-proxy.exe
```

**预期输出**:
```
 _    _ _         ______
| |  (_) |       (_____ \
| |  _| |_ ___   _____) )_ __  ___  _  _  _   _
| | | |  _/ _ \ |  ____/ '__/ / _ \| |/ || | | |
| |___| | ||  __/| |    | |  | (_) |  /  | |_| |
|_______)_|\___/ |_|    |_|   \___/|_|    \__  |
                                          (____/
kite-proxy - Kubernetes API Forwarding Proxy

Starting kite-proxy on port 8090
Kite server URL: http://localhost:8080
Auto-sync started with interval: 5m0s
kite-proxy listening on :8090
UI available at http://localhost:8090/ui/
K8s proxy endpoint: http://localhost:8090/proxy/<cluster-name>/
```

## 步骤 3：验证连接

```powershell
# 检查健康状态
curl http://localhost:8090/healthz
# 输出: {"status":"ok"}

# 检查详细状态
curl http://localhost:8090/api/status | ConvertFrom-Json
# 输出:
# {
#   "status": "ok",
#   "configured": true,
#   "cachedClusters": [],
#   "syncEnabled": true,
#   "lastSyncError": null
# }
```

## 步骤 4：列出可用集群

```powershell
curl http://localhost:8090/api/clusters | ConvertFrom-Json
```

**示例输出**:
```json
{
  "clusters": [
    {
      "name": "dev-cluster",
      "cached": false
    },
    {
      "name": "prod-cluster",
      "cached": false
    }
  ]
}
```

## 步骤 5：生成 kubectl 配置

### 方式 A：通过 API 下载

```powershell
curl http://localhost:8090/api/kubeconfig -o kubeconfig-kite-proxy.yaml
```

### 方式 B：通过 Web UI 下载

1. 打开 http://localhost:8090/ui/
2. 进入 **Usage** 页面
3. 点击 **Download Kubeconfig** 按钮

## 步骤 6：使用 kubectl

```powershell
# 设置 KUBECONFIG 环境变量
$env:KUBECONFIG = "kubeconfig-kite-proxy.yaml"

# 查看可用的 context
kubectl config get-contexts

# 输出示例:
# CURRENT   NAME                    CLUSTER                USER               NAMESPACE
# *         kite-proxy-dev-cluster  kite-proxy-dev-cluster kite-proxy-user
#           kite-proxy-prod-cluster kite-proxy-prod-cluster kite-proxy-user

# 使用 kubectl 访问集群
kubectl get nodes
kubectl get pods -A
kubectl get namespaces
```

### 切换集群

```powershell
# 列出所有 context
kubectl config get-contexts

# 切换到 prod-cluster
kubectl config use-context kite-proxy-prod-cluster

# 再次查看资源
kubectl get pods -n production
```

## 步骤 7：Web UI 使用

### Configuration 页面

访问 http://localhost:8090/ui/ → **Configuration**

- 查看当前配置（API Key 已脱敏）
- 修改 kite URL 或 API Key
- 测试连接

### Clusters 页面

访问 http://localhost:8090/ui/ → **Clusters**

- 查看所有可访问的集群
- 查看缓存状态
- 点击 **Pre-warm** 按钮提前加载 kubeconfig

### Usage 页面

访问 http://localhost:8090/ui/ → **Usage**

- 查看使用说明
- 复制示例 kubectl 命令
- 下载 kubeconfig 文件

## 高级用法

### 预热集群缓存

在首次使用 kubectl 之前，可以预热缓存以加快响应速度：

```powershell
# 预热特定集群
curl -X POST http://localhost:8090/api/cache/dev-cluster

# 输出: {"message":"cluster \"dev-cluster\" warmed up"}
```

### 清除缓存

如果 kubeconfig 更新或遇到认证问题：

```powershell
# 清除所有缓存
curl -X DELETE http://localhost:8090/api/cache

# 输出: {"message":"cache cleared"}
```

### 手动触发同步

```powershell
# 手动触发与 kite 服务器的同步
curl -X POST http://localhost:8090/api/sync

# 输出: {"message":"sync completed successfully"}
```

### 直接指定 server（不使用 kubeconfig）

```powershell
kubectl --server=http://localhost:8090/proxy/dev-cluster `
        --insecure-skip-tls-verify `
        get pods -n default
```

## 常见问题排查

### 问题 1：无法连接到 kite 服务器

**症状**:
```
curl http://localhost:8090/api/status
# lastSyncError: "request to kite server failed: ..."
```

**解决方法**:
1. 检查 kite 服务器是否运行：`curl http://localhost:8080/healthz`
2. 检查 kite URL 是否正确
3. 检查网络连接和防火墙设置

### 问题 2：认证失败

**症状**:
```
curl http://localhost:8090/api/clusters
# {"error":"kite server returned 401: ..."}
```

**解决方法**:
1. 检查 API Key 是否正确
2. 在 kite 管理界面验证 API Key 是否有效
3. 确认对应角色有 `allowProxy: true`

### 问题 3：找不到集群

**症状**:
```
curl http://localhost:8090/api/clusters
# {"clusters":[]}
```

**解决方法**:
1. 检查 API Key 对应的角色是否有集群访问权限
2. 确认角色的 `clusters` 字段不为空
3. 确认 `allowProxy: true` 已设置
4. 在 kite 管理界面检查用户的角色分配

### 问题 4：kubectl 命令失败

**症状**:
```powershell
kubectl get pods
# Error from server (Forbidden): pods is forbidden: ...
```

**解决方法**:
1. 这是 Kubernetes RBAC 权限问题，不是 kite-proxy 的问题
2. 检查目标集群中的 ServiceAccount 权限
3. 在 kite 中检查 `proxyNamespaces` 和 `verbs` 配置

### 问题 5：缓存未更新

**症状**:
集群配置已更改，但 kubectl 仍使用旧的配置

**解决方法**:
```powershell
# 清除缓存
curl -X DELETE http://localhost:8090/api/cache

# 或重启 kite-proxy
```

## 生产环境部署建议

### 1. 使用 HTTPS

在 kite-proxy 前放置反向代理（如 nginx）处理 TLS：

```nginx
server {
    listen 443 ssl;
    server_name kite-proxy.example.com;
    
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    
    location / {
        proxy_pass http://localhost:8090;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

### 2. 使用 systemd 服务

创建 `/etc/systemd/system/kite-proxy.service`：

```ini
[Unit]
Description=kite-proxy - Kubernetes API Forwarding Proxy
After=network.target

[Service]
Type=simple
User=kite-proxy
WorkingDirectory=/opt/kite-proxy
Environment="KITE_URL=https://kite.example.com"
Environment="KITE_API_KEY=kite123-production-key"
Environment="PORT=8090"
ExecStart=/opt/kite-proxy/kite-proxy
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

启动服务：
```bash
sudo systemctl daemon-reload
sudo systemctl enable kite-proxy
sudo systemctl start kite-proxy
sudo systemctl status kite-proxy
```

### 3. 使用 Docker

创建 `Dockerfile`：

```dockerfile
FROM golang:1.25 AS builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o kite-proxy .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/kite-proxy /usr/local/bin/
EXPOSE 8090
ENTRYPOINT ["kite-proxy"]
```

运行：
```bash
docker build -t kite-proxy .
docker run -d \
  --name kite-proxy \
  -p 8090:8090 \
  -e KITE_URL=https://kite.example.com \
  -e KITE_API_KEY=kite123-production-key \
  kite-proxy
```

### 4. 监控和日志

使用 klog 的日志级别：

```powershell
# 详细日志（级别 2）
.\kite-proxy.exe -v=2

# 非常详细（级别 3）
.\kite-proxy.exe -v=3
```

收集指标：
```powershell
# 定期检查状态
while ($true) {
    $status = Invoke-RestMethod http://localhost:8090/api/status
    Write-Host "$(Get-Date) - Cached: $($status.cachedClusters.Count), Error: $($status.lastSyncError)"
    Start-Sleep -Seconds 60
}
```

## 下一步

- 阅读 [ARCHITECTURE.md](ARCHITECTURE.md) 了解内部架构
- 查看 [VERIFICATION_PLAN.md](VERIFICATION_PLAN.md) 进行完整测试
- 参考 [README.md](README.md) 了解所有 API 端点

## 需要帮助？

- 提交 Issue: https://github.com/zxh326/kite-proxy/issues
- 查看日志: `.\kite-proxy.exe -v=2`
- 检查 kite 服务器状态
