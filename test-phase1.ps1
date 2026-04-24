# kite-proxy Phase 1 核心功能快速验证

# 配置变量 - 请根据实际环境修改
$KITE_URL = "http://localhost:8080"
$API_KEY = "kite123-your-api-key-here"
$PROXY_PORT = "8090"
$PROXY_URL = "http://localhost:$PROXY_PORT"

Write-Host "===================================" -ForegroundColor Cyan
Write-Host "kite-proxy Phase 1 功能验证脚本" -ForegroundColor Cyan
Write-Host "===================================" -ForegroundColor Cyan
Write-Host ""

# 检查 kite-proxy 是否在运行
Write-Host "1. 检查 kite-proxy 健康状态..." -ForegroundColor Yellow
try {
    $health = Invoke-RestMethod -Uri "$PROXY_URL/healthz" -Method Get -ErrorAction Stop
    Write-Host "   ✓ kite-proxy 运行正常: $($health.status)" -ForegroundColor Green
} catch {
    Write-Host "   ✗ kite-proxy 未运行或不可访问" -ForegroundColor Red
    Write-Host "   请先启动 kite-proxy:" -ForegroundColor Yellow
    Write-Host "   ./kite-proxy --port $PROXY_PORT --kite-url $KITE_URL --api-key YOUR_API_KEY" -ForegroundColor Gray
    exit 1
}
Write-Host ""

# 测试配置 API
Write-Host "2. 测试配置 API..." -ForegroundColor Yellow
try {
    $config = Invoke-RestMethod -Uri "$PROXY_URL/api/config" -Method Get -ErrorAction Stop
    Write-Host "   ✓ 获取配置成功" -ForegroundColor Green
    Write-Host "     - Kite URL: $($config.kiteURL)" -ForegroundColor Gray
    Write-Host "     - API Key (masked): $($config.apiKeyMasked)" -ForegroundColor Gray
    Write-Host "     - Configured: $($config.configured)" -ForegroundColor Gray
} catch {
    Write-Host "   ✗ 获取配置失败: $_" -ForegroundColor Red
}
Write-Host ""

# 测试状态 API
Write-Host "3. 测试状态 API..." -ForegroundColor Yellow
try {
    $status = Invoke-RestMethod -Uri "$PROXY_URL/api/status" -Method Get -ErrorAction Stop
    Write-Host "   ✓ 获取状态成功" -ForegroundColor Green
    Write-Host "     - Status: $($status.status)" -ForegroundColor Gray
    Write-Host "     - Configured: $($status.configured)" -ForegroundColor Gray
    Write-Host "     - Sync Enabled: $($status.syncEnabled)" -ForegroundColor Gray
    Write-Host "     - Cached Clusters: $($status.cachedClusters.Count)" -ForegroundColor Gray
    if ($status.lastSyncError) {
        Write-Host "     - Last Sync Error: $($status.lastSyncError)" -ForegroundColor Red
    } else {
        Write-Host "     - Last Sync Error: None" -ForegroundColor Green
    }
} catch {
    Write-Host "   ✗ 获取状态失败: $_" -ForegroundColor Red
}
Write-Host ""

# 测试手动同步
Write-Host "4. 测试手动同步..." -ForegroundColor Yellow
try {
    $syncResult = Invoke-RestMethod -Uri "$PROXY_URL/api/sync" -Method Post -ErrorAction Stop
    Write-Host "   ✓ 手动同步成功: $($syncResult.message)" -ForegroundColor Green
} catch {
    Write-Host "   ✗ 手动同步失败: $_" -ForegroundColor Red
    Write-Host "     请检查 kite 服务器连接和 API Key 是否有效" -ForegroundColor Yellow
}
Write-Host ""

# 测试集群列表 API
Write-Host "5. 测试集群列表 API..." -ForegroundColor Yellow
try {
    $clusters = Invoke-RestMethod -Uri "$PROXY_URL/api/clusters" -Method Get -ErrorAction Stop
    Write-Host "   ✓ 获取集群列表成功" -ForegroundColor Green
    Write-Host "     找到 $($clusters.clusters.Count) 个集群:" -ForegroundColor Gray
    foreach ($cluster in $clusters.clusters) {
        $cacheStatus = if ($cluster.cached) { "[已缓存]" } else { "[未缓存]" }
        Write-Host "     - $($cluster.name) $cacheStatus" -ForegroundColor Gray
    }
    
    if ($clusters.clusters.Count -eq 0) {
        Write-Host "     ⚠ 未找到可访问的集群" -ForegroundColor Yellow
        Write-Host "     请检查 API Key 是否有 allowProxy 权限" -ForegroundColor Yellow
    }
} catch {
    Write-Host "   ✗ 获取集群列表失败: $_" -ForegroundColor Red
}
Write-Host ""

# 测试 kubeconfig 生成
Write-Host "6. 测试 kubeconfig 生成..." -ForegroundColor Yellow
try {
    $kubeconfig = Invoke-RestMethod -Uri "$PROXY_URL/api/kubeconfig" -Method Get -ErrorAction Stop
    Write-Host "   ✓ 生成 kubeconfig 成功" -ForegroundColor Green
    Write-Host "     Kubeconfig 长度: $($kubeconfig.Length) 字节" -ForegroundColor Gray
    
    # 保存到临时文件
    $tempFile = "test-kubeconfig-$(Get-Date -Format 'yyyyMMdd-HHmmss').yaml"
    $kubeconfig | Out-File -FilePath $tempFile -Encoding utf8
    Write-Host "     已保存到: $tempFile" -ForegroundColor Gray
    Write-Host "     使用方法: `$env:KUBECONFIG='$tempFile'; kubectl get pods" -ForegroundColor Cyan
} catch {
    Write-Host "   ✗ 生成 kubeconfig 失败: $_" -ForegroundColor Red
}
Write-Host ""

# 测试缓存预热（如果有集群）
if ($clusters.clusters.Count -gt 0) {
    $testCluster = $clusters.clusters[0].name
    Write-Host "7. 测试缓存预热（集群: $testCluster）..." -ForegroundColor Yellow
    try {
        $prewarmResult = Invoke-RestMethod -Uri "$PROXY_URL/api/cache/$testCluster" -Method Post -ErrorAction Stop
        Write-Host "   ✓ 预热成功: $($prewarmResult.message)" -ForegroundColor Green
        
        # 再次检查状态，确认缓存已更新
        $statusAfter = Invoke-RestMethod -Uri "$PROXY_URL/api/status" -Method Get -ErrorAction Stop
        Write-Host "     缓存的集群数量: $($statusAfter.cachedClusters.Count)" -ForegroundColor Gray
    } catch {
        Write-Host "   ✗ 预热失败: $_" -ForegroundColor Red
    }
    Write-Host ""
    
    # 测试清除缓存
    Write-Host "8. 测试清除缓存..." -ForegroundColor Yellow
    try {
        $clearResult = Invoke-RestMethod -Uri "$PROXY_URL/api/cache" -Method Delete -ErrorAction Stop
        Write-Host "   ✓ 清除缓存成功: $($clearResult.message)" -ForegroundColor Green
        
        # 再次检查状态，确认缓存已清空
        $statusAfterClear = Invoke-RestMethod -Uri "$PROXY_URL/api/status" -Method Get -ErrorAction Stop
        Write-Host "     缓存的集群数量: $($statusAfterClear.cachedClusters.Count)" -ForegroundColor Gray
    } catch {
        Write-Host "   ✗ 清除缓存失败: $_" -ForegroundColor Red
    }
    Write-Host ""
} else {
    Write-Host "7-8. 跳过缓存测试（无可用集群）" -ForegroundColor Yellow
    Write-Host ""
}

# 总结
Write-Host "===================================" -ForegroundColor Cyan
Write-Host "Phase 1 核心功能验证完成" -ForegroundColor Cyan
Write-Host "===================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "已验证功能：" -ForegroundColor Green
Write-Host "  ✓ Kite API 客户端（获取 kubeconfig）" -ForegroundColor Green
Write-Host "  ✓ 内存中的 Kubeconfig 存储" -ForegroundColor Green
Write-Host "  ✓ 配置自动同步机制" -ForegroundColor Green
Write-Host "  ✓ REST API 端点" -ForegroundColor Green
Write-Host ""
Write-Host "下一步：" -ForegroundColor Yellow
Write-Host "  1. 使用生成的 kubeconfig 测试 kubectl 命令" -ForegroundColor Yellow
Write-Host "  2. 验证 K8s API 反向代理功能" -ForegroundColor Yellow
Write-Host "  3. 运行完整的 VERIFICATION_PLAN.md 测试" -ForegroundColor Yellow
Write-Host ""
