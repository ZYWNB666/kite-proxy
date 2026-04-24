package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zxh326/kite-proxy/pkg/api"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

// kubeconfigEntry holds an in-memory kubeconfig for a single cluster.
// The raw kubeconfig YAML is intentionally NOT stored here; only the
// parsed rest.Config is kept so that secrets are not retained as plain text.
type kubeconfigEntry struct {
	restConfig *rest.Config
}

// kubeconfigCache is an in-memory store of cluster → *rest.Config.
// kubeconfigs are NEVER written to disk.
type kubeconfigCache struct {
	mu      sync.RWMutex
	entries map[string]*kubeconfigEntry
}

// globalCache is the singleton in-memory kubeconfig store.
var globalCache = &kubeconfigCache{
	entries: make(map[string]*kubeconfigEntry),
}

// Get returns the rest.Config for clusterName, fetching it from the kite
// server if it is not already cached.
func (c *kubeconfigCache) Get(clusterName string) (*rest.Config, error) {
	c.mu.RLock()
	if entry, ok := c.entries[clusterName]; ok {
		c.mu.RUnlock()
		return entry.restConfig, nil
	}
	c.mu.RUnlock()

	// Not cached – fetch from kite server, then store under write lock.
	// Multiple goroutines might race here; only the first to acquire the
	// write lock will insert; subsequent ones will find the value already set.
	restCfg, err := fetchRestConfig(clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch kubeconfig for cluster %q: %w", clusterName, err)
	}

	c.mu.Lock()
	// Double-check: another goroutine may have inserted the entry while we
	// were fetching (after we released the read lock above).
	if entry, ok := c.entries[clusterName]; ok {
		c.mu.Unlock()
		return entry.restConfig, nil
	}
	c.entries[clusterName] = &kubeconfigEntry{restConfig: restCfg}
	c.mu.Unlock()

	klog.Infof("Loaded kubeconfig for cluster %q into memory cache", clusterName)
	return restCfg, nil
}

// Clear removes all cached kubeconfigs (e.g. after config change).
func (c *kubeconfigCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*kubeconfigEntry)
	klog.Info("Kubeconfig cache cleared")
}

// ClearCluster removes the cached kubeconfig for a single cluster.
func (c *kubeconfigCache) ClearCluster(clusterName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, clusterName)
}

// ListCached returns the names of all clusters that have a cached kubeconfig.
func (c *kubeconfigCache) ListCached() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	names := make([]string, 0, len(c.entries))
	for name := range c.entries {
		names = append(names, name)
	}
	return names
}

// fetchRestConfig calls the kite server's proxy kubeconfig endpoint and returns
// a parsed rest.Config for the requested cluster.
// The raw YAML is used only transiently inside this function and is not stored.
func fetchRestConfig(clusterName string) (*rest.Config, error) {
	cfg := GetConfig()

	// Create API client
	client := api.NewClient(cfg.KiteURL, cfg.APIKey)

	// Fetch kubeconfig with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clusterKubeconfig, err := client.GetClusterKubeconfig(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	// Parse kubeconfig YAML into rest.Config
	restConfig, err := api.ParseKubeconfig(clusterKubeconfig.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kubeconfig for cluster %q: %w", clusterName, err)
	}

	// Raw YAML (clusterKubeconfig.Kubeconfig) is discarded here – only the parsed config is kept.
	return restConfig, nil
}

// FetchAvailableClusters calls the kite server and returns the list of
// cluster names that are available for proxying (based on RBAC permissions).
func FetchAvailableClusters() ([]ClusterInfo, error) {
	cfg := GetConfig()

	// Create API client
	client := api.NewClient(cfg.KiteURL, cfg.APIKey)

	// Fetch all available kubeconfigs with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.GetKubeconfigs(ctx, "")
	if err != nil {
		return nil, err
	}

	// Build ClusterInfo list with cache status
	clusters := make([]ClusterInfo, 0, len(resp.Clusters))
	for _, cl := range resp.Clusters {
		cached := false
		globalCache.mu.RLock()
		_, cached = globalCache.entries[cl.Name]
		globalCache.mu.RUnlock()

		clusters = append(clusters, ClusterInfo{
			Name:   cl.Name,
			Cached: cached,
		})
	}

	return clusters, nil
}

// ClusterInfo is a lightweight struct used in API responses.
type ClusterInfo struct {
	Name   string `json:"name"`
	Cached bool   `json:"cached"`
}
