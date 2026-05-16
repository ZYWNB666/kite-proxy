package server

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zxh326/kite-proxy/pkg/api"
	"k8s.io/klog/v2"
)

// handleGetConfig returns the current config (API key is masked).
func handleGetConfig(c *gin.Context) {
	cfg := GetConfig()
	maskedKey := ""
	if cfg.APIKey != "" {
		if len(cfg.APIKey) > 8 {
			maskedKey = cfg.APIKey[:4] + "****" + cfg.APIKey[len(cfg.APIKey)-4:]
		} else {
			maskedKey = "****"
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"kiteURL":      cfg.KiteURL,
		"apiKeyMasked": maskedKey,
		"configured":   cfg.KiteURL != "" && cfg.APIKey != "",
	})
}

// handleSetConfig updates the kite server URL and API key.
func handleSetConfig(c *gin.Context) {
	var req struct {
		KiteURL string `json:"kiteURL" binding:"required"`
		APIKey  string `json:"apiKey"  binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	SetConfig(Config{
		Port:    GetConfig().Port,
		KiteURL: req.KiteURL,
		APIKey:  req.APIKey,
	})

	klog.Infof("Configuration updated: kiteURL=%s", req.KiteURL)
	c.JSON(http.StatusOK, gin.H{"message": "configuration saved"})
}

// handleListClusters fetches available clusters from the kite server.
func handleListClusters(c *gin.Context) {
	clusters, err := FetchAvailableClusters()
	if err != nil {
		respondAPIError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"clusters": clusters})
}

// handleGetKubeconfig returns a local kubeconfig that points kubectl at
// this kite-proxy instance.  Users can pipe the output into a file and
// use it with KUBECONFIG env variable.
func handleGetKubeconfig(c *gin.Context) {
	clusters, err := FetchAvailableClusters()
	if err != nil {
		respondAPIError(c, err)
		return
	}

	cfg := GetConfig()
	port := cfg.Port
	if port == "" {
		port = "8090"
	}
	baseURL := fmt.Sprintf("http://localhost:%s", port)

	// Extract cluster names
	clusterNames := make([]string, 0, len(clusters))
	for _, cl := range clusters {
		clusterNames = append(clusterNames, cl.Name)
	}

	// Build kubeconfig YAML using the api package
	kubeconfigYAML := api.BuildKubeconfigYAML(clusterNames, baseURL)

	c.Header("Content-Type", "application/x-yaml")
	c.Header("Content-Disposition", `attachment; filename="kubeconfig-kite-proxy.yaml"`)
	c.String(http.StatusOK, kubeconfigYAML)
}

// handleClearCache removes all cached kubeconfigs from memory.
func handleClearCache(c *gin.Context) {
	globalCache.Clear()
	c.JSON(http.StatusOK, gin.H{"message": "cache cleared"})
}

// handleStatus returns health and status information.
func handleStatus(c *gin.Context) {
	cfg := GetConfig()
	lastSyncErr, syncRunning := GetSyncStatus()

	status := gin.H{
		"status":         "ok",
		"configured":     cfg.KiteURL != "" && cfg.APIKey != "",
		"cachedClusters": globalCache.ListCached(),
		"syncEnabled":    syncRunning,
	}

	if lastSyncErr != nil {
		status["lastSyncError"] = lastSyncErr.Error()
	} else {
		status["lastSyncError"] = nil
	}

	c.JSON(http.StatusOK, status)
}

// handleTriggerSync manually triggers a synchronization with kite server.
func handleTriggerSync(c *gin.Context) {
	if globalSyncer == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "syncer not initialized"})
		return
	}

	err := globalSyncer.SyncNow()
	if err != nil {
		respondAPIError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "sync completed successfully"})
}

// handlePrewarm fetches (and caches) the kubeconfig for a specific cluster.
func handlePrewarm(c *gin.Context) {
	clusterName := c.Param("cluster")
	if clusterName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cluster name is required"})
		return
	}
	globalCache.ClearCluster(clusterName)
	if err := globalCache.RefreshCluster(clusterName); err != nil {
		respondAPIError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("cluster %q warmed up", clusterName)})
}
