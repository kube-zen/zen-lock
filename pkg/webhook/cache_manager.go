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

	"k8s.io/apimachinery/pkg/types"
)

// CacheManager manages ZenLock cache instances for invalidation
type CacheManager struct {
	mu     sync.RWMutex
	caches []*ZenLockCache
}

var globalCacheManager = &CacheManager{
	caches: make([]*ZenLockCache, 0),
}

// RegisterCache registers a cache instance with the global manager
func RegisterCache(cache *ZenLockCache) {
	globalCacheManager.mu.Lock()
	defer globalCacheManager.mu.Unlock()
	globalCacheManager.caches = append(globalCacheManager.caches, cache)
}

// UnregisterCache removes a cache instance from the global manager
func UnregisterCache(cache *ZenLockCache) {
	globalCacheManager.mu.Lock()
	defer globalCacheManager.mu.Unlock()
	for i, c := range globalCacheManager.caches {
		if c == cache {
			globalCacheManager.caches = append(globalCacheManager.caches[:i], globalCacheManager.caches[i+1:]...)
			break
		}
	}
}

// InvalidateZenLock invalidates a ZenLock in all registered caches
func InvalidateZenLock(key types.NamespacedName) {
	globalCacheManager.mu.RLock()
	defer globalCacheManager.mu.RUnlock()
	for _, cache := range globalCacheManager.caches {
		cache.Invalidate(key)
	}
}

// InvalidateAll invalidates all entries in all registered caches
func InvalidateAll() {
	globalCacheManager.mu.RLock()
	defer globalCacheManager.mu.RUnlock()
	for _, cache := range globalCacheManager.caches {
		cache.InvalidateAll()
	}
}

