package desktop

import (
	"sync"

	"k8s.io/client-go/rest"
)

// KubeconfigCache 是桌面应用专用的缓存
type KubeconfigCache struct {
	mu      sync.RWMutex
	entries map[string]*rest.Config
}

// NewKubeconfigCache 创建新的缓存实例
func NewKubeconfigCache() *KubeconfigCache {
	return &KubeconfigCache{
		entries: make(map[string]*rest.Config),
	}
}

// Get 获取指定集群的 kubeconfig
func (c *KubeconfigCache) Get(clusterName string) (*rest.Config, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cfg, ok := c.entries[clusterName]
	return cfg, ok
}

// Set 设置指定集群的 kubeconfig
func (c *KubeconfigCache) Set(clusterName string, cfg *rest.Config) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[clusterName] = cfg
}

// Has 检查是否已缓存指定集群
func (c *KubeconfigCache) Has(clusterName string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, ok := c.entries[clusterName]
	return ok
}

// Clear 清除所有缓存
func (c *KubeconfigCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*rest.Config)
}

// List 列出所有已缓存的集群名称
func (c *KubeconfigCache) List() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.entries))
	for name := range c.entries {
		names = append(names, name)
	}
	return names
}
