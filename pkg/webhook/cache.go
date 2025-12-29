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

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.zen.io/v1alpha1"
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
	}

	// Start background cleanup goroutine
	go cache.cleanup()

	return cache
}

// Get retrieves a ZenLock from cache if available and not expired
func (c *ZenLockCache) Get(key types.NamespacedName) (*securityv1alpha1.ZenLock, bool) {
	if c == nil {
		return nil, false
	}
	
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.cache[key]
	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.expiresAt) {
		return nil, false
	}

	entry.lastAccess = time.Now()
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
func (c *ZenLockCache) cleanup() {
	ticker := time.NewTicker(c.cleanupInt)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for key, entry := range c.cache {
				if now.After(entry.expiresAt) {
					delete(c.cache, key)
				}
			}
			c.mu.Unlock()
		case <-c.stopCh:
			return
		}
	}
}

// Stop stops the background cleanup goroutine
func (c *ZenLockCache) Stop() {
	close(c.stopCh)
}

// Size returns the current cache size
func (c *ZenLockCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}

