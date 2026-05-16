package api

import (
	"context"
	"time"
)

// ClusterNamespaces describes the namespaces a key may proxy for a cluster.
type ClusterNamespaces struct {
	Name       string   `json:"name"`
	Namespaces []string `json:"namespaces"`
}

type proxyNamespacesResponse struct {
	Clusters []ClusterNamespaces `json:"clusters"`
}

// FetchProxyNamespaces 获取指定集群（或所有集群）允许 proxy 的 namespace 列表。
// cluster 为空时返回所有有权限的集群。
func FetchProxyNamespaces(apiEndpoint, apiKey, cluster string) ([]ClusterNamespaces, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := NewClient(apiEndpoint, apiKey)
	return client.GetProxyNamespaces(ctx, cluster)
}
