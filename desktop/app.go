package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/zxh326/kite-proxy/pkg/api"
	"k8s.io/klog/v2"
)

// Config holds the desktop app configuration
type Config struct {
	KiteURL string
	APIKey  string
}

// PersistedConfig is the on-disk representation of the config
type PersistedConfig struct {
	KiteURL  string `json:"kiteURL"`
	APIKey   string `json:"apiKey"`
	Language string `json:"language"` // "en" | "zh"
	Theme    string `json:"theme"`    // "light" | "dark"
}

// configFilePath returns the path to the persisted config file
func configFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".kite-proxy", "config.json")
}

// ClusterInfo describes a cluster
type ClusterInfo struct {
	Name   string `json:"name"`
	Cached bool   `json:"cached"`
}

// App 是桌面应用的结构体
type App struct {
	ctx            context.Context
	config         *Config
	cache          *KubeconfigCache
	client         *api.Client
	portManager    *PortForwardManager
	trayIcon       []byte
	uiLanguage     string
	uiTheme        string
	authMu         sync.Mutex
	lastAuthCheck  time.Time
	stopAuthLoop   chan struct{}
}

// NewApp 创建一个新的 App 应用实例
func NewApp(trayIcon []byte) *App {
	app := &App{
		cache:        NewKubeconfigCache(),
		trayIcon:     trayIcon,
		stopAuthLoop: make(chan struct{}),
	}
	app.portManager = NewPortForwardManager(app)
	return app
}

// Startup 在应用启动时调用
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	klog.Info("Desktop app starting...")
	// 从磁盘加载配置
	a.loadConfigFromDisk()
	// 启动 API key 定期校验
	go a.startAuthLoop()
	// 启动系统托盘
	a.startTray()
}

// loadConfigFromDisk 从用户家目录加载配置
func (a *App) loadConfigFromDisk() {
	path := configFilePath()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			klog.Errorf("Failed to read config file: %v", err)
		}
		return
	}
	var cfg PersistedConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		klog.Errorf("Failed to parse config file: %v", err)
		return
	}
	// 加载 UI 偏好
	if cfg.Language != "" {
		a.uiLanguage = cfg.Language
	}
	if cfg.Theme != "" {
		a.uiTheme = cfg.Theme
	}
	if cfg.KiteURL != "" && cfg.APIKey != "" {
		a.config = &Config{
			KiteURL: cfg.KiteURL,
			APIKey:  cfg.APIKey,
		}
		a.client = api.NewClient(cfg.KiteURL, cfg.APIKey)
		klog.Infof("Loaded config from disk: kiteURL=%s", cfg.KiteURL)
	}
}

// saveConfigToDisk 将配置持久化到用户家目录
func (a *App) saveConfigToDisk() {
	path := configFilePath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		klog.Errorf("Failed to create config directory: %v", err)
		return
	}
	cfg := PersistedConfig{
		Language: a.uiLanguage,
		Theme:    a.uiTheme,
	}
	if a.config != nil {
		cfg.KiteURL = a.config.KiteURL
		cfg.APIKey = a.config.APIKey
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		klog.Errorf("Failed to marshal config: %v", err)
		return
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		klog.Errorf("Failed to write config file: %v", err)
	} else {
		klog.Info("Config saved to disk")
	}
}

// Shutdown 在应用关闭时调用
func (a *App) Shutdown(ctx context.Context) {
	klog.Info("Desktop app shutting down...")
	// 停止 auth 校验循环
	select {
	case <-a.stopAuthLoop:
	default:
		close(a.stopAuthLoop)
	}
	// 停止所有端口转发
	if a.portManager != nil {
		a.portManager.StopAll()
	}
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

	// 持久化配置到磁盘
	a.saveConfigToDisk()

	// 发送通知到前端
	runtime.EventsEmit(a.ctx, "config:updated")

	return nil
}

// checkAuth 校验 API key 是否有效（60秒内相同结果复用缓存）
func (a *App) checkAuth() error {
	if a.client == nil {
		return fmt.Errorf("not configured")
	}
	a.authMu.Lock()
	shouldCheck := time.Since(a.lastAuthCheck) > 60*time.Second
	a.authMu.Unlock()
	if !shouldCheck {
		return nil
	}
	if err := a.client.Ping(context.Background()); err != nil {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "auth:unauthorized")
		}
		return fmt.Errorf("API key validation failed: %w", err)
	}
	a.authMu.Lock()
	a.lastAuthCheck = time.Now()
	a.authMu.Unlock()
	return nil
}

// startAuthLoop 每 6 分钟主动校验一次 API key
func (a *App) startAuthLoop() {
	ticker := time.NewTicker(6 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if a.client == nil {
				continue
			}
			if err := a.client.Ping(context.Background()); err != nil {
				klog.Warningf("Periodic auth check failed: %v", err)
				a.authMu.Lock()
				a.lastAuthCheck = time.Time{} // 清除缓存，强制下次重新校验
				a.authMu.Unlock()
				if a.ctx != nil {
					runtime.EventsEmit(a.ctx, "auth:unauthorized")
				}
			} else {
				a.authMu.Lock()
				a.lastAuthCheck = time.Now()
				a.authMu.Unlock()
			}
		case <-a.stopAuthLoop:
			return
		}
	}
}

// OpenBrowser 使用系统默认浏览器打开 URL
func (a *App) OpenBrowser(url string) {
	runtime.BrowserOpenURL(a.ctx, url)
}

// GetUIPrefs 获取 UI 偏好（语言、主题）
func (a *App) GetUIPrefs() map[string]string {
	lang := a.uiLanguage
	if lang == "" {
		lang = "en"
	}
	theme := a.uiTheme
	if theme == "" {
		theme = "light"
	}
	return map[string]string{
		"language": lang,
		"theme":    theme,
	}
}

// SetUIPrefs 保存 UI 偏好到磁盘
func (a *App) SetUIPrefs(language, theme string) error {
	a.uiLanguage = language
	a.uiTheme = theme
	a.saveConfigToDisk()
	return nil
}

// ListClusters 获取可用的集群列表
func (a *App) ListClusters() ([]ClusterInfo, error) {
	if err := a.checkAuth(); err != nil {
		return nil, err
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

// ========== 端口映射管理 ==========

// AddPortMapping 添加端口映射
func (a *App) AddPortMapping(cluster, namespace, resourceType, resourceName string, remotePort, localPort int) (*PortMapping, error) {
	if a.client == nil {
		return nil, fmt.Errorf("not configured")
	}

	return a.portManager.AddMapping(cluster, namespace, resourceType, resourceName, remotePort, localPort)
}

// RemovePortMapping 删除端口映射
func (a *App) RemovePortMapping(id string) error {
	return a.portManager.RemoveMapping(id)
}

// ListPortMappings 列出所有端口映射
func (a *App) ListPortMappings() []*PortMapping {
	return a.portManager.ListMappings()
}

// StartPortMapping 启动端口映射
func (a *App) StartPortMapping(id string) error {
	return a.portManager.StartMapping(id)
}

// StopPortMapping 停止端口映射
func (a *App) StopPortMapping(id string) error {
	return a.portManager.StopMapping(id)
}
