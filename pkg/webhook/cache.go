/*
Copyright 2025 Kube-ZEN Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhook

import (
	"sync"
	"time"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
	"github.com/kube-zen/zen-lock/pkg/controller/metrics"
	"k8s.io/apimachinery/pkg/types"
)

// ZenLockCache provides thread-safe caching for ZenLock CRDs
// to reduce API server load and improve webhook response times
type ZenLockCache struct {
	cache      map[types.NamespacedName]*cacheEntry
	mu         sync.RWMutex
	ttl        time.Duration
	cleanupInt time.Duration
	stopCh     chan struct{}
	hits       int64         // Cache hit counter
	misses     int64         // Cache miss counter
	metricsCh  chan struct{} // Channel to trigger metrics update
}

type cacheEntry struct {
	zenlock    *securityv1alpha1.ZenLock
	expiresAt  time.Time
	lastAccess time.Time
}

// NewZenLockCache creates a new ZenLock cache with the specified TTL
func NewZenLockCache(ttl time.Duration) *ZenLockCache {
	cache := &ZenLockCache{
		cache:      make(map[types.NamespacedName]*cacheEntry),
		ttl:        ttl,
		cleanupInt: ttl / 2, // Cleanup every half TTL
		stopCh:     make(chan struct{}),
		metricsCh:  make(chan struct{}, 1), // Buffered channel for metrics updates
	}

	// Start background cleanup goroutine
	go cache.cleanup()

	// Start background metrics update goroutine
	go cache.updateMetricsLoop()

	return cache
}

// Get retrieves a ZenLock from cache if available and not expired
func (c *ZenLockCache) Get(key types.NamespacedName) (*securityv1alpha1.ZenLock, bool) {
	if c == nil {
		return nil, false
	}

	c.mu.RLock()
	entry, exists := c.cache[key]
	if !exists {
		c.mu.RUnlock()
		c.recordMiss()
		return nil, false
	}

	// Check if expired
	now := time.Now()
	if now.After(entry.expiresAt) {
		c.mu.RUnlock()
		c.recordMiss()
		return nil, false
	}
	c.mu.RUnlock()

	// Update lastAccess with write lock (race condition fix)
	c.mu.Lock()
	if entry, stillExists := c.cache[key]; stillExists && !now.After(entry.expiresAt) {
		entry.lastAccess = now
	}
	c.mu.Unlock()

	c.recordHit()
	return entry.zenlock.DeepCopy(), true
}

// Set stores a ZenLock in the cache
func (c *ZenLockCache) Set(key types.NamespacedName, zenlock *securityv1alpha1.ZenLock) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[key] = &cacheEntry{
		zenlock:    zenlock.DeepCopy(),
		expiresAt:  time.Now().Add(c.ttl),
		lastAccess: time.Now(),
	}
}

// Invalidate removes a specific entry from the cache
func (c *ZenLockCache) Invalidate(key types.NamespacedName) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.cache, key)
}

// InvalidateAll clears the entire cache
func (c *ZenLockCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[types.NamespacedName]*cacheEntry)
}

// cleanup periodically removes expired entries
// Optimized for Go 1.25: collect expired keys first, then delete in batch
func (c *ZenLockCache) cleanup() {
	ticker := time.NewTicker(c.cleanupInt)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			// Collect expired keys first (more efficient than deleting during iteration)
			expiredKeys := make([]types.NamespacedName, 0)
			for key, entry := range c.cache {
				if now.After(entry.expiresAt) {
					expiredKeys = append(expiredKeys, key)
				}
			}
			// Delete expired entries in batch
			for _, key := range expiredKeys {
				delete(c.cache, key)
			}
			c.mu.Unlock()
		case <-c.stopCh:
			return
		}
	}
}

// Stop stops the background cleanup goroutine
func (c *ZenLockCache) Stop() {
	select {
	case <-c.stopCh:
		// Already stopped, do nothing
	default:
		close(c.stopCh)
	}
}

// Size returns the current cache size
func (c *ZenLockCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}

// recordHit increments the hit counter and triggers metrics update
func (c *ZenLockCache) recordHit() {
	c.mu.Lock()
	c.hits++
	c.mu.Unlock()
	// Trigger metrics update (non-blocking)
	select {
	case c.metricsCh <- struct{}{}:
	default:
		// Channel full, skip this update
	}
}

// recordMiss increments the miss counter and triggers metrics update
func (c *ZenLockCache) recordMiss() {
	c.mu.Lock()
	c.misses++
	c.mu.Unlock()
	// Trigger metrics update (non-blocking)
	select {
	case c.metricsCh <- struct{}{}:
	default:
		// Channel full, skip this update
	}
}

// updateMetricsLoop periodically updates cache metrics
func (c *ZenLockCache) updateMetricsLoop() {
	ticker := time.NewTicker(30 * time.Second) // Update metrics every 30 seconds
	defer ticker.Stop()

	// Update immediately on first run
	c.updateMetrics()

	for {
		select {
		case <-ticker.C:
			c.updateMetrics()
		case <-c.metricsCh:
			// Triggered by hit/miss, but we'll update on next ticker to avoid excessive updates
			// The ticker will handle the actual update
		case <-c.stopCh:
			return
		}
	}
}

// updateMetrics updates the cache size and hit rate metrics
func (c *ZenLockCache) updateMetrics() {
	c.mu.RLock()
	size := len(c.cache)
	hits := c.hits
	misses := c.misses
	c.mu.RUnlock()

	metrics.UpdateCacheMetrics(size, hits, misses)
}
