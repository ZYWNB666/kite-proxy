package server

import (
	"context"
	"sync"
	"time"

	"github.com/zxh326/kite-proxy/pkg/api"
	"k8s.io/klog/v2"
)

// Syncer handles automatic synchronization of cluster list and cache warming.
type Syncer struct {
	interval    time.Duration
	stopCh      chan struct{}
	stoppedCh   chan struct{}
	mu          sync.Mutex
	running     bool
	lastSyncErr error
}

// NewSyncer creates a new automatic syncer with the specified interval.
func NewSyncer(interval time.Duration) *Syncer {
	return &Syncer{
		interval:  interval,
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}
}

// Start begins the automatic synchronization loop in a background goroutine.
// It periodically checks connectivity to kite server and refreshes cluster list.
func (s *Syncer) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go s.syncLoop()
	klog.Infof("Auto-sync started with interval: %v", s.interval)
}

// Stop gracefully stops the synchronization loop.
func (s *Syncer) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	close(s.stopCh)
	<-s.stoppedCh

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()

	klog.Info("Auto-sync stopped")
}

// LastSyncError returns the error from the last sync attempt, or nil if successful.
func (s *Syncer) LastSyncError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastSyncErr
}

// syncLoop runs the periodic synchronization.
func (s *Syncer) syncLoop() {
	defer close(s.stoppedCh)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run an immediate sync on startup (after a short delay to let config settle)
	time.Sleep(2 * time.Second)
	s.performSync()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.performSync()
		}
	}
}

// performSync attempts to sync with kite server and update cache status.
func (s *Syncer) performSync() {
	cfg := GetConfig()

	// Skip sync if not configured
	if cfg.KiteURL == "" || cfg.APIKey == "" {
		klog.V(3).Info("Skipping sync: kite server not configured")
		s.mu.Lock()
		s.lastSyncErr = nil
		s.mu.Unlock()
		return
	}

	klog.V(2).Info("Starting automatic sync with kite server")

	// Create API client
	client := api.NewClient(cfg.KiteURL, cfg.APIKey)

	// Ping kite server to check connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.Ping(ctx)
	if err != nil {
		klog.Warningf("Sync failed: %v", err)
		s.mu.Lock()
		s.lastSyncErr = err
		s.mu.Unlock()
		return
	}

	klog.V(2).Info("Sync successful: kite server is reachable")
	s.mu.Lock()
	s.lastSyncErr = nil
	s.mu.Unlock()
}

// SyncNow triggers an immediate synchronization.
func (s *Syncer) SyncNow() error {
	cfg := GetConfig()

	if cfg.KiteURL == "" || cfg.APIKey == "" {
		return nil // not an error, just not configured
	}

	client := api.NewClient(cfg.KiteURL, cfg.APIKey)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.Ping(ctx)
	s.mu.Lock()
	s.lastSyncErr = err
	s.mu.Unlock()

	return err
}

// Global syncer instance
var globalSyncer *Syncer

// InitSyncer creates and starts the global auto-syncer.
// Call this once during application startup.
func InitSyncer(interval time.Duration) {
	if globalSyncer != nil {
		globalSyncer.Stop()
	}
	globalSyncer = NewSyncer(interval)
	globalSyncer.Start()
}

// StopSyncer stops the global auto-syncer.
func StopSyncer() {
	if globalSyncer != nil {
		globalSyncer.Stop()
	}
}

// GetSyncStatus returns the status of the last sync attempt.
func GetSyncStatus() (lastError error, running bool) {
	if globalSyncer == nil {
		return nil, false
	}
	return globalSyncer.LastSyncError(), globalSyncer.running
}
