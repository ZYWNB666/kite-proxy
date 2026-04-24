package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"k8s.io/klog/v2"
)

// Client is the HTTP client for communicating with the kite server.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new kite API client.
func NewClient(kiteURL, apiKey string) *Client {
	return &Client{
		baseURL: kiteURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ClusterKubeconfig represents a single cluster's kubeconfig returned from kite.
type ClusterKubeconfig struct {
	Name       string `json:"name"`
	Kubeconfig string `json:"kubeconfig"`
}

// KubeconfigResponse is the JSON structure returned by /api/v1/proxy/kubeconfig.
type KubeconfigResponse struct {
	Clusters []ClusterKubeconfig `json:"clusters"`
}

// GetKubeconfigs fetches all available kubeconfigs from the kite server.
// If clusterName is not empty, it filters to only that specific cluster.
func (c *Client) GetKubeconfigs(ctx context.Context, clusterName string) (*KubeconfigResponse, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("kite server URL is not configured")
	}
	if c.apiKey == "" {
		return nil, fmt.Errorf("kite API key is not configured")
	}

	// Build the request URL
	endpoint := fmt.Sprintf("%s/api/v1/proxy/kubeconfig", c.baseURL)
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid kite URL: %w", err)
	}

	// Add cluster filter if specified
	if clusterName != "" {
		q := u.Query()
		q.Set("cluster", clusterName)
		u.RawQuery = q.Encode()
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication header (kite expects API key directly, not "Bearer <key>")
	req.Header.Set("Authorization", c.apiKey)
	req.Header.Set("Accept", "application/json")

	klog.V(2).Infof("Fetching kubeconfig from kite: %s", u.String())

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to kite server failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Handle non-200 responses
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kite server returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var result KubeconfigResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode kite response: %w", err)
	}

	klog.V(2).Infof("Successfully fetched %d cluster(s) from kite", len(result.Clusters))
	return &result, nil
}

// GetClusterKubeconfig fetches the kubeconfig for a specific cluster.
// Returns an error if the cluster is not found or not accessible.
func (c *Client) GetClusterKubeconfig(ctx context.Context, clusterName string) (*ClusterKubeconfig, error) {
	resp, err := c.GetKubeconfigs(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	// Find the requested cluster in the response
	for _, cluster := range resp.Clusters {
		if cluster.Name == clusterName {
			return &cluster, nil
		}
	}

	return nil, fmt.Errorf("cluster %q not found in kite response", clusterName)
}

// ListClusters returns just the names of available clusters (without kubeconfigs).
// This is useful for populating UI dropdowns without fetching full credentials.
func (c *Client) ListClusters(ctx context.Context) ([]string, error) {
	resp, err := c.GetKubeconfigs(ctx, "")
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(resp.Clusters))
	for _, cluster := range resp.Clusters {
		names = append(names, cluster.Name)
	}

	return names, nil
}

// Ping verifies connectivity to the kite server by attempting to list clusters.
// This can be used for health checks and configuration validation.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.ListClusters(ctx)
	if err != nil {
		return fmt.Errorf("kite server is unreachable or authentication failed: %w", err)
	}
	return nil
}

// SetTimeout updates the HTTP client timeout.
func (c *Client) SetTimeout(timeout time.Duration) {
	c.httpClient.Timeout = timeout
}

// UpdateConfig updates the base URL and API key.
// This is useful when configuration changes at runtime.
func (c *Client) UpdateConfig(kiteURL, apiKey string) {
	c.baseURL = kiteURL
	c.apiKey = apiKey
	klog.V(2).Infof("Kite client configuration updated: %s", kiteURL)
}
