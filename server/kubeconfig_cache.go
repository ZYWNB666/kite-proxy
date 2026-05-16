package server

import (
	"context"
	"fmt"
	"sort"
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
	restConfig        *rest.Config
	allowedNamespaces map[string]struct{}
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

	if err := c.RefreshCluster(clusterName); err != nil {
		return nil, fmt.Errorf("failed to fetch kubeconfig for cluster %q: %w", clusterName, err)
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[clusterName]
	if !ok {
		return nil, fmt.Errorf("cluster %q not found in cache after refresh", clusterName)
	}
	return entry.restConfig, nil
}

// GetAllowedNamespaces returns the allowed namespaces for a cluster.
func (c *kubeconfigCache) GetAllowedNamespaces(clusterName string) ([]string, error) {
	c.mu.RLock()
	entry, ok := c.entries[clusterName]
	c.mu.RUnlock()
	if !ok {
		if err := c.RefreshCluster(clusterName); err != nil {
			return nil, err
		}
		c.mu.RLock()
		entry = c.entries[clusterName]
		c.mu.RUnlock()
	}
	if entry == nil {
		return nil, fmt.Errorf("cluster %q not found", clusterName)
	}

	namespaces := make([]string, 0, len(entry.allowedNamespaces))
	for namespace := range entry.allowedNamespaces {
		namespaces = append(namespaces, namespace)
	}
	sort.Strings(namespaces)
	return namespaces, nil
}

// IsNamespaceAllowed reports whether a namespace is allowed for a cluster.
// Empty namespace means the request is cluster-scoped and should pass through.
func (c *kubeconfigCache) IsNamespaceAllowed(clusterName, namespace string) (bool, error) {
	if namespace == "" {
		return true, nil
	}

	allowedNamespaces, err := c.GetAllowedNamespaces(clusterName)
	if err != nil {
		return false, err
	}
	for _, allowed := range allowedNamespaces {
		if allowed == namespace {
			return true, nil
		}
	}
	return false, nil
}

// RefreshAll refreshes kubeconfigs and namespace permissions for all clusters atomically.
func (c *kubeconfigCache) RefreshAll() error {
	entries, err := fetchClusterEntries("")
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.entries = entries
	c.mu.Unlock()

	klog.Infof("Refreshed %d cluster kubeconfig(s) and namespace permission set(s)", len(entries))
	return nil
}

// RefreshCluster refreshes a single cluster atomically.
func (c *kubeconfigCache) RefreshCluster(clusterName string) error {
	entries, err := fetchClusterEntries(clusterName)
	if err != nil {
		return err
	}

	entry, ok := entries[clusterName]
	if !ok {
		return fmt.Errorf("cluster %q not found in kite response", clusterName)
	}

	c.mu.Lock()
	c.entries[clusterName] = entry
	c.mu.Unlock()

	klog.Infof("Refreshed kubeconfig and namespace permissions for cluster %q", clusterName)
	return nil
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

// fetchClusterEntries calls the kite server and atomically builds in-memory
// kubeconfig + allowed namespace entries.
func fetchClusterEntries(clusterName string) (map[string]*kubeconfigEntry, error) {
	cfg := GetConfig()
	if cfg.KiteURL == "" || cfg.APIKey == "" {
		return nil, fmt.Errorf("kite server is not configured")
	}

	client := api.NewClient(cfg.KiteURL, cfg.APIKey)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	kubeconfigResp, err := client.GetKubeconfigs(ctx, clusterName)
	if err != nil {
		return nil, err
	}
	proxyNamespaces, err := client.GetProxyNamespaces(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	namespaceMap := make(map[string]map[string]struct{}, len(proxyNamespaces))
	for _, cluster := range proxyNamespaces {
		allowed := make(map[string]struct{}, len(cluster.Namespaces))
		for _, namespace := range cluster.Namespaces {
			allowed[namespace] = struct{}{}
		}
		namespaceMap[cluster.Name] = allowed
	}

	entries := make(map[string]*kubeconfigEntry, len(kubeconfigResp.Clusters))
	for _, cluster := range kubeconfigResp.Clusters {
		restConfig, err := api.ParseKubeconfig(cluster.Kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to parse kubeconfig for cluster %q: %w", cluster.Name, err)
		}
		entries[cluster.Name] = &kubeconfigEntry{
			restConfig:        restConfig,
			allowedNamespaces: namespaceMap[cluster.Name],
		}
		if entries[cluster.Name].allowedNamespaces == nil {
			entries[cluster.Name].allowedNamespaces = make(map[string]struct{})
		}
	}

	return entries, nil
}

// FetchAvailableClusters calls the kite server and returns the list of
// cluster names that are available for proxying (based on RBAC permissions).
func FetchAvailableClusters() ([]ClusterInfo, error) {
	if err := globalCache.RefreshAll(); err != nil {
		return nil, err
	}

	globalCache.mu.RLock()
	defer globalCache.mu.RUnlock()

	clusters := make([]ClusterInfo, 0, len(globalCache.entries))
	for name := range globalCache.entries {
		clusters = append(clusters, ClusterInfo{
			Name:   name,
			Cached: true,
		})
	}
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Name < clusters[j].Name
	})

	return clusters, nil
}

// ClusterInfo is a lightweight struct used in API responses.
type ClusterInfo struct {
	Name   string `json:"name"`
	Cached bool   `json:"cached"`
}
