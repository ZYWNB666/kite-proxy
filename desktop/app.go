package desktop

import (
	"context"
	"encoding/json"
	"errors"
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
	KiteURL      string                 `json:"kiteURL"`
	APIKey       string                 `json:"apiKey"`
	Language     string                 `json:"language"`     // "en" | "zh"
	Theme        string                 `json:"theme"`        // "light" | "dark"
	PortMappings []PersistedPortMapping `json:"portMappings"` // 持久化的端口映射
}

// PersistedPortMapping 是端口映射的持久化表示（不包含运行时状态）
type PersistedPortMapping struct {
	ID           string `json:"id"`
	Cluster      string `json:"cluster"`
	Namespace    string `json:"namespace"`
	ResourceType string `json:"resourceType"`
	ResourceName string `json:"resourceName"`
	RemotePort   int    `json:"remotePort"`
	LocalPort    int    `json:"localPort"`
	AutoStart    bool   `json:"autoStart"` // 是否在启动时自动开始转发
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
	ctx           context.Context
	config        *Config
	cache         *KubeconfigCache
	client        *api.Client
	portManager   *PortForwardManager
	trayIcon      []byte
	uiLanguage    string
	uiTheme       string
	authMu        sync.Mutex
	lastAuthCheck time.Time
	stopAuthLoop  chan struct{}
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
	// 恢复持久化的端口映射
	a.restorePortMappings()
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
	// 加载端口映射（在 Startup 中恢复）
	if len(cfg.PortMappings) > 0 {
		klog.Infof("Loaded %d persisted port mappings", len(cfg.PortMappings))
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
	// 保存端口映射
	if a.portManager != nil {
		mappings := a.portManager.ListMappings()
		persistedMappings := make([]PersistedPortMapping, 0, len(mappings))
		for _, m := range mappings {
			persistedMappings = append(persistedMappings, PersistedPortMapping{
				ID:           m.ID,
				Cluster:      m.Cluster,
				Namespace:    m.Namespace,
				ResourceType: m.ResourceType,
				ResourceName: m.ResourceName,
				RemotePort:   m.RemotePort,
				LocalPort:    m.LocalPort,
				AutoStart:    m.Status == "running", // 保存当前状态
			})
		}
		cfg.PortMappings = persistedMappings
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
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

// restorePortMappings 恢复持久化的端口映射
func (a *App) restorePortMappings() {
	path := configFilePath()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var cfg PersistedConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return
	}
	if len(cfg.PortMappings) == 0 {
		return
	}
	if a.config == nil || a.client == nil {
		klog.Info("Skipping port mapping restoration: not configured yet")
		return
	}

	klog.Infof("Restoring %d port mappings...", len(cfg.PortMappings))
	for _, pm := range cfg.PortMappings {
		// 添加映射（但不自动启动，保持停止状态）
		mapping, err := a.portManager.addMappingWithoutAutoStart(
			pm.Cluster, pm.Namespace, pm.ResourceType, pm.ResourceName,
			pm.RemotePort, pm.LocalPort,
		)
		if err != nil {
			klog.Errorf("Failed to restore mapping %s: %v", pm.ID, err)
			continue
		}
		// 如果之前是运行状态，尝试启动
		if pm.AutoStart {
			if err := a.portManager.StartMapping(mapping.ID); err != nil {
				klog.Warningf("Failed to auto-start mapping %s: %v", mapping.ID, err)
			}
		}
	}
	klog.Info("Port mappings restored")
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
		a.emitAuthError(err)
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
				a.emitAuthError(err)
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

func (a *App) emitAuthError(err error) {
	if a.ctx == nil {
		return
	}

	code := "kite_unreachable"
	message := fmt.Sprintf("无法连接到 Kite 服务端：%v", err)

	switch {
	case errors.Is(err, api.ErrUnauthorized):
		code = "unauthorized"
		message = "API key 无效或已过期，请重新配置"
		runtime.EventsEmit(a.ctx, "auth:unauthorized")
	case errors.Is(err, api.ErrProxyForbidden):
		code = "proxy_forbidden"
		message = "当前 API key 没有访问该集群/命名空间的代理权限"
	}

	runtime.EventsEmit(a.ctx, "auth:error", map[string]string{
		"code":    code,
		"message": message,
	})
}

// ========== 端口映射管理 ==========

// AddPortMapping 添加端口映射
func (a *App) AddPortMapping(cluster, namespace, resourceType, resourceName string, remotePort, localPort int) (*PortMapping, error) {
	if a.client == nil {
		return nil, fmt.Errorf("not configured")
	}

	mapping, err := a.portManager.AddMapping(cluster, namespace, resourceType, resourceName, remotePort, localPort)
	if err != nil {
		return nil, err
	}

	// 持久化映射到磁盘
	a.saveConfigToDisk()

	return mapping, nil
}

// RemovePortMapping 删除端口映射
func (a *App) RemovePortMapping(id string) error {
	err := a.portManager.RemoveMapping(id)
	if err != nil {
		return err
	}

	// 持久化到磁盘
	a.saveConfigToDisk()

	return nil
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
