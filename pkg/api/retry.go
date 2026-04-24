package api

import (
	"context"
	"fmt"
	"math"
	"time"

	"k8s.io/klog/v2"
)

// RetryConfig defines retry behavior for API requests.
type RetryConfig struct {
	MaxRetries     int           // Maximum number of retry attempts
	InitialBackoff time.Duration // Initial backoff duration
	MaxBackoff     time.Duration // Maximum backoff duration
	Multiplier     float64       // Backoff multiplier
}

// DefaultRetryConfig returns a sensible default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 500 * time.Millisecond,
		MaxBackoff:     10 * time.Second,
		Multiplier:     2.0,
	}
}

// RetryableClient wraps a Client with automatic retry logic.
type RetryableClient struct {
	*Client
	retryConfig RetryConfig
}

// NewRetryableClient creates a new client with retry capabilities.
func NewRetryableClient(kiteURL, apiKey string, retryConfig RetryConfig) *RetryableClient {
	return &RetryableClient{
		Client:      NewClient(kiteURL, apiKey),
		retryConfig: retryConfig,
	}
}

// GetKubeconfigs fetches kubeconfigs with automatic retry on transient failures.
func (c *RetryableClient) GetKubeconfigs(ctx context.Context, clusterName string) (*KubeconfigResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= c.retryConfig.MaxRetries; attempt++ {
		resp, err := c.Client.GetKubeconfigs(ctx, clusterName)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Don't retry on context cancellation or timeout
		if ctx.Err() != nil {
			return nil, fmt.Errorf("request cancelled or timed out: %w", err)
		}

		// Don't retry on final attempt
		if attempt == c.retryConfig.MaxRetries {
			break
		}

		// Calculate backoff duration with exponential increase
		backoff := calculateBackoff(
			c.retryConfig.InitialBackoff,
			c.retryConfig.MaxBackoff,
			c.retryConfig.Multiplier,
			attempt,
		)

		klog.V(2).Infof("Kite API request failed (attempt %d/%d): %v. Retrying in %v...",
			attempt+1, c.retryConfig.MaxRetries+1, err, backoff)

		// Wait before retrying
		select {
		case <-time.After(backoff):
			// Continue to next attempt
		case <-ctx.Done():
			return nil, fmt.Errorf("request cancelled during retry: %w", ctx.Err())
		}
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", c.retryConfig.MaxRetries+1, lastErr)
}

// GetClusterKubeconfig fetches a specific cluster's kubeconfig with retry.
func (c *RetryableClient) GetClusterKubeconfig(ctx context.Context, clusterName string) (*ClusterKubeconfig, error) {
	resp, err := c.GetKubeconfigs(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	for _, cluster := range resp.Clusters {
		if cluster.Name == clusterName {
			return &cluster, nil
		}
	}

	return nil, fmt.Errorf("cluster %q not found in kite response", clusterName)
}

// ListClusters returns cluster names with retry.
func (c *RetryableClient) ListClusters(ctx context.Context) ([]string, error) {
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

// Ping verifies connectivity with retry.
func (c *RetryableClient) Ping(ctx context.Context) error {
	_, err := c.ListClusters(ctx)
	return err
}

// calculateBackoff computes the backoff duration for a given attempt.
func calculateBackoff(initial, max time.Duration, multiplier float64, attempt int) time.Duration {
	backoff := float64(initial) * math.Pow(multiplier, float64(attempt))
	if backoff > float64(max) {
		backoff = float64(max)
	}
	return time.Duration(backoff)
}
