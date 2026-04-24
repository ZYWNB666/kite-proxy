package desktop

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/zxh326/kite-proxy/pkg/api"
	"k8s.io/klog/v2"
)

// Config holds the desktop app configuration
type Config struct {
	KiteURL string
	APIKey  string
}

// ClusterInfo describes a cluster
type ClusterInfo struct {
	Name   string `json:"name"`
	Cached bool   `json:"cached"`
}

// App 是桌面应用的结构体
type App struct {
	ctx    context.Context
	config *Config
	cache  *KubeconfigCache
	client *api.Client
}

// NewApp 创建一个新的 App 应用实例
func NewApp() *App {
	return &App{
		cache: NewKubeconfigCache(),
	}
}

// Startup 在应用启动时调用
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	klog.Info("Desktop app starting...")
	// 桌面版不保存配置到磁盘，每次启动需要重新配置
}

// Shutdown 在应用关闭时调用
func (a *App) Shutdown(ctx context.Context) {
	klog.Info("Desktop app shutting down...")
	// 清除敏感数据
	a.cache.Clear()
}

// GetConfig 获取当前配置
func (a *App) GetConfig() map[string]interface{} {
	if a.config == nil {
		return map[string]interface{}{
			"kiteURL":      "",
			"apiKeyMasked": "",
			"configured":   false,
		}
	}

	maskedKey := ""
	if a.config.APIKey != "" {
		if len(a.config.APIKey) > 8 {
			maskedKey = a.config.APIKey[:4] + "****" + a.config.APIKey[len(a.config.APIKey)-4:]
		} else {
			maskedKey = "****"
		}
	}

	return map[string]interface{}{
		"kiteURL":      a.config.KiteURL,
		"apiKeyMasked": maskedKey,
		"configured":   true,
	}
}

// SetConfig 设置 kite 服务器配置
func (a *App) SetConfig(kiteURL, apiKey string) error {
	if kiteURL == "" || apiKey == "" {
		return fmt.Errorf("kite URL and API key are required")
	}

	a.config = &Config{
		KiteURL: kiteURL,
		APIKey:  apiKey,
	}

	// 创建新的客户端
	a.client = api.NewClient(kiteURL, apiKey)

	// 清除旧的缓存
	a.cache.Clear()

	klog.Infof("Configuration updated: kiteURL=%s", kiteURL)

	// 发送通知到前端
	runtime.EventsEmit(a.ctx, "config:updated")

	return nil
}

// ListClusters 获取可用的集群列表
func (a *App) ListClusters() ([]ClusterInfo, error) {
	if a.client == nil {
		return nil, fmt.Errorf("not configured")
	}

	ctx := context.Background()
	resp, err := a.client.GetKubeconfigs(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch clusters: %w", err)
	}

	clusters := make([]ClusterInfo, 0, len(resp.Clusters))
	for _, cl := range resp.Clusters {
		cached := a.cache.Has(cl.Name)
		clusters = append(clusters, ClusterInfo{
			Name:   cl.Name,
			Cached: cached,
		})
	}

	return clusters, nil
}

// PrewarmCluster 预加载指定集群的 kubeconfig
func (a *App) PrewarmCluster(clusterName string) error {
	if a.client == nil {
		return fmt.Errorf("not configured")
	}

	ctx := context.Background()
	clusterKC, err := a.client.GetClusterKubeconfig(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to fetch kubeconfig: %w", err)
	}

	// 解析并缓存
	restConfig, err := api.ParseKubeconfig(clusterKC.Kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	a.cache.Set(clusterName, restConfig)
	klog.Infof("Prewarmed cluster: %s", clusterName)

	// 通知前端缓存已更新
	runtime.EventsEmit(a.ctx, "cache:updated", clusterName)

	return nil
}

// ClearCache 清除所有缓存
func (a *App) ClearCache() {
	a.cache.Clear()
	klog.Info("Cache cleared")
	runtime.EventsEmit(a.ctx, "cache:cleared")
}

// TestConnection 测试与 kite 服务器的连接
func (a *App) TestConnection() error {
	if a.client == nil {
		return fmt.Errorf("not configured")
	}

	ctx := context.Background()
	err := a.client.Ping(ctx)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	return nil
}

// GetKubeconfigYAML 生成本地 kubeconfig YAML
func (a *App) GetKubeconfigYAML() (string, error) {
	clusters, err := a.ListClusters()
	if err != nil {
		return "", err
	}

	clusterNames := make([]string, 0, len(clusters))
	for _, cl := range clusters {
		clusterNames = append(clusterNames, cl.Name)
	}

	// 桌面版使用本地代理服务器（如果启动了的话）
	baseURL := "http://localhost:8090"

	return api.BuildKubeconfigYAML(clusterNames, baseURL), nil
}

// ShowNotification 显示系统通知
func (a *App) ShowNotification(title, message string) {
	runtime.EventsEmit(a.ctx, "notification", map[string]string{
		"title":   title,
		"message": message,
	})
}
