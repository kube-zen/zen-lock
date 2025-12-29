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

func TestZenLockCache_Get(t *testing.T) {
	cache := NewZenLockCache(5 * time.Minute)
	defer cache.Stop()

	key := types.NamespacedName{Namespace: "default", Name: "test-zenlock"}
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": "value1",
			},
		},
	}

	// Test Get on empty cache
	_, found := cache.Get(key)
	if found {
		t.Error("Expected cache miss for empty cache")
	}

	// Test Get after Set
	cache.Set(key, zenlock)
	retrieved, found := cache.Get(key)
	if !found {
		t.Error("Expected cache hit after Set")
	}
	if retrieved.Name != zenlock.Name {
		t.Errorf("Expected name %s, got %s", zenlock.Name, retrieved.Name)
	}

	// Test Get with nil cache
	var nilCache *ZenLockCache
	_, found = nilCache.Get(key)
	if found {
		t.Error("Expected false for nil cache")
	}
}

func TestZenLockCache_Set(t *testing.T) {
	cache := NewZenLockCache(5 * time.Minute)
	defer cache.Stop()

	key := types.NamespacedName{Namespace: "default", Name: "test-zenlock"}
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": "value1",
			},
		},
	}

	cache.Set(key, zenlock)
	if cache.Size() != 1 {
		t.Errorf("Expected cache size 1, got %d", cache.Size())
	}

	// Test Set overwrites existing entry
	zenlock2 := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key2": "value2",
			},
		},
	}
	cache.Set(key, zenlock2)
	if cache.Size() != 1 {
		t.Errorf("Expected cache size 1 after overwrite, got %d", cache.Size())
	}
	retrieved, found := cache.Get(key)
	if !found {
		t.Error("Expected cache hit after overwrite")
	}
	if retrieved.Spec.EncryptedData["key2"] != "value2" {
		t.Error("Expected overwritten value")
	}
}

func TestZenLockCache_Invalidate(t *testing.T) {
	cache := NewZenLockCache(5 * time.Minute)
	defer cache.Stop()

	key1 := types.NamespacedName{Namespace: "default", Name: "zenlock1"}
	key2 := types.NamespacedName{Namespace: "default", Name: "zenlock2"}

	zenlock1 := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{Name: "zenlock1", Namespace: "default"},
		Spec:       securityv1alpha1.ZenLockSpec{EncryptedData: map[string]string{"key1": "value1"}},
	}
	zenlock2 := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{Name: "zenlock2", Namespace: "default"},
		Spec:       securityv1alpha1.ZenLockSpec{EncryptedData: map[string]string{"key2": "value2"}},
	}

	cache.Set(key1, zenlock1)
	cache.Set(key2, zenlock2)

	if cache.Size() != 2 {
		t.Errorf("Expected cache size 2, got %d", cache.Size())
	}

	// Invalidate one entry
	cache.Invalidate(key1)

	if cache.Size() != 1 {
		t.Errorf("Expected cache size 1 after invalidate, got %d", cache.Size())
	}

	_, found := cache.Get(key1)
	if found {
		t.Error("Expected cache miss for invalidated entry")
	}

	_, found = cache.Get(key2)
	if !found {
		t.Error("Expected cache hit for non-invalidated entry")
	}
}

func TestZenLockCache_InvalidateAll(t *testing.T) {
	cache := NewZenLockCache(5 * time.Minute)
	defer cache.Stop()

	key1 := types.NamespacedName{Namespace: "default", Name: "zenlock1"}
	key2 := types.NamespacedName{Namespace: "default", Name: "zenlock2"}

	zenlock1 := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{Name: "zenlock1", Namespace: "default"},
		Spec:       securityv1alpha1.ZenLockSpec{EncryptedData: map[string]string{"key1": "value1"}},
	}
	zenlock2 := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{Name: "zenlock2", Namespace: "default"},
		Spec:       securityv1alpha1.ZenLockSpec{EncryptedData: map[string]string{"key2": "value2"}},
	}

	cache.Set(key1, zenlock1)
	cache.Set(key2, zenlock2)

	if cache.Size() != 2 {
		t.Errorf("Expected cache size 2, got %d", cache.Size())
	}

	// Invalidate all
	cache.InvalidateAll()

	if cache.Size() != 0 {
		t.Errorf("Expected cache size 0 after InvalidateAll, got %d", cache.Size())
	}

	_, found := cache.Get(key1)
	if found {
		t.Error("Expected cache miss after InvalidateAll")
	}

	_, found = cache.Get(key2)
	if found {
		t.Error("Expected cache miss after InvalidateAll")
	}
}

func TestZenLockCache_Size(t *testing.T) {
	cache := NewZenLockCache(5 * time.Minute)
	defer cache.Stop()

	if cache.Size() != 0 {
		t.Errorf("Expected initial cache size 0, got %d", cache.Size())
	}

	key1 := types.NamespacedName{Namespace: "default", Name: "zenlock1"}
	key2 := types.NamespacedName{Namespace: "default", Name: "zenlock2"}

	zenlock1 := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{Name: "zenlock1", Namespace: "default"},
		Spec:       securityv1alpha1.ZenLockSpec{EncryptedData: map[string]string{"key1": "value1"}},
	}
	zenlock2 := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{Name: "zenlock2", Namespace: "default"},
		Spec:       securityv1alpha1.ZenLockSpec{EncryptedData: map[string]string{"key2": "value2"}},
	}

	cache.Set(key1, zenlock1)
	if cache.Size() != 1 {
		t.Errorf("Expected cache size 1, got %d", cache.Size())
	}

	cache.Set(key2, zenlock2)
	if cache.Size() != 2 {
		t.Errorf("Expected cache size 2, got %d", cache.Size())
	}

	cache.Invalidate(key1)
	if cache.Size() != 1 {
		t.Errorf("Expected cache size 1 after invalidate, got %d", cache.Size())
	}
}

func TestZenLockCache_Stop(t *testing.T) {
	cache := NewZenLockCache(100 * time.Millisecond)
	
	key := types.NamespacedName{Namespace: "default", Name: "test-zenlock"}
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{Name: "test-zenlock", Namespace: "default"},
		Spec:       securityv1alpha1.ZenLockSpec{EncryptedData: map[string]string{"key1": "value1"}},
	}

	cache.Set(key, zenlock)

	// Stop should not panic
	cache.Stop()

	// Wait a bit to ensure cleanup goroutine has stopped
	time.Sleep(200 * time.Millisecond)

	// Stop should be idempotent
	cache.Stop()
}

func TestZenLockCache_Expiration(t *testing.T) {
	cache := NewZenLockCache(50 * time.Millisecond)
	defer cache.Stop()

	key := types.NamespacedName{Namespace: "default", Name: "test-zenlock"}
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{Name: "test-zenlock", Namespace: "default"},
		Spec:       securityv1alpha1.ZenLockSpec{EncryptedData: map[string]string{"key1": "value1"}},
	}

	cache.Set(key, zenlock)

	// Should be available immediately
	_, found := cache.Get(key)
	if !found {
		t.Error("Expected cache hit immediately after Set")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired
	_, found = cache.Get(key)
	if found {
		t.Error("Expected cache miss after expiration")
	}
}

