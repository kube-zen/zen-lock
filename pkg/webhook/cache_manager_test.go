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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
)

func TestRegisterCache(t *testing.T) {
	cache1 := NewZenLockCache(5 * time.Minute)
	defer cache1.Stop()

	RegisterCache(cache1)

	// Verify cache is registered by invalidating a key
	key := types.NamespacedName{Name: "test", Namespace: "default"}
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
	}
	cache1.Set(key, zenlock)

	// Invalidate via manager
	InvalidateZenLock(key)

	// Verify cache entry was invalidated
	_, found := cache1.Get(key)
	if found {
		t.Error("Expected cache entry to be invalidated")
	}
}

func TestUnregisterCache(t *testing.T) {
	cache1 := NewZenLockCache(5 * time.Minute)
	defer cache1.Stop()

	cache2 := NewZenLockCache(5 * time.Minute)
	defer cache2.Stop()

	RegisterCache(cache1)
	RegisterCache(cache2)

	// Unregister cache1
	UnregisterCache(cache1)

	// Set entries in both caches
	key := types.NamespacedName{Name: "test", Namespace: "default"}
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
	}
	cache1.Set(key, zenlock)
	cache2.Set(key, zenlock)

	// Invalidate via manager - should only affect cache2
	InvalidateZenLock(key)

	// cache1 should still have the entry (unregistered)
	_, found1 := cache1.Get(key)
	if !found1 {
		t.Error("Expected cache1 to still have entry after unregistering")
	}

	// cache2 should have entry invalidated
	_, found2 := cache2.Get(key)
	if found2 {
		t.Error("Expected cache2 entry to be invalidated")
	}
}

func TestInvalidateZenLock(t *testing.T) {
	cache1 := NewZenLockCache(5 * time.Minute)
	defer cache1.Stop()

	cache2 := NewZenLockCache(5 * time.Minute)
	defer cache2.Stop()

	RegisterCache(cache1)
	RegisterCache(cache2)

	key := types.NamespacedName{Name: "test", Namespace: "default"}
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
	}

	cache1.Set(key, zenlock)
	cache2.Set(key, zenlock)

	// Invalidate via manager
	InvalidateZenLock(key)

	// Both caches should have entry invalidated
	_, found1 := cache1.Get(key)
	if found1 {
		t.Error("Expected cache1 entry to be invalidated")
	}

	_, found2 := cache2.Get(key)
	if found2 {
		t.Error("Expected cache2 entry to be invalidated")
	}
}

func TestInvalidateAll(t *testing.T) {
	cache1 := NewZenLockCache(5 * time.Minute)
	defer cache1.Stop()

	cache2 := NewZenLockCache(5 * time.Minute)
	defer cache2.Stop()

	RegisterCache(cache1)
	RegisterCache(cache2)

	key1 := types.NamespacedName{Name: "test1", Namespace: "default"}
	key2 := types.NamespacedName{Name: "test2", Namespace: "default"}

	zenlock1 := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test1",
			Namespace: "default",
		},
	}
	zenlock2 := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test2",
			Namespace: "default",
		},
	}

	cache1.Set(key1, zenlock1)
	cache1.Set(key2, zenlock2)
	cache2.Set(key1, zenlock1)
	cache2.Set(key2, zenlock2)

	// Invalidate all via manager
	InvalidateAll()

	// All entries should be invalidated
	_, found1 := cache1.Get(key1)
	if found1 {
		t.Error("Expected cache1 key1 to be invalidated")
	}

	_, found2 := cache1.Get(key2)
	if found2 {
		t.Error("Expected cache1 key2 to be invalidated")
	}

	_, found3 := cache2.Get(key1)
	if found3 {
		t.Error("Expected cache2 key1 to be invalidated")
	}

	_, found4 := cache2.Get(key2)
	if found4 {
		t.Error("Expected cache2 key2 to be invalidated")
	}
}

