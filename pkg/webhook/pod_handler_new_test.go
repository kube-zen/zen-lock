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
	"os"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func TestNewPodHandler(t *testing.T) {
	// Save original value
	originalKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("ZEN_LOCK_PRIVATE_KEY", originalKey)
		} else {
			os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")
		}
	}()

	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(securityv1alpha1.AddToScheme(scheme))

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Test without private key
	os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")
	_, err := NewPodHandler(client, scheme)
	if err == nil {
		t.Error("Expected error when ZEN_LOCK_PRIVATE_KEY is not set")
	}

	// Test with private key
	os.Setenv("ZEN_LOCK_PRIVATE_KEY", "AGE-SECRET-1EXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLE")
	handler, err := NewPodHandler(client, scheme)
	if err != nil {
		t.Fatalf("Failed to create PodHandler: %v", err)
	}
	if handler == nil {
		t.Error("PodHandler should not be nil")
	}
	if handler.Client == nil {
		t.Error("PodHandler.Client should not be nil")
	}
	if handler.crypto == nil {
		t.Error("PodHandler.crypto should not be nil")
	}
	if handler.privateKey == "" {
		t.Error("PodHandler.privateKey should not be empty")
	}
	if handler.cache == nil {
		t.Error("PodHandler.cache should not be nil")
	}

	// Test with custom cache TTL
	os.Setenv("ZEN_LOCK_CACHE_TTL", "10m")
	handler2, err := NewPodHandler(client, scheme)
	if err != nil {
		t.Fatalf("Failed to create PodHandler with custom TTL: %v", err)
	}
	if handler2.cache == nil {
		t.Error("PodHandler.cache should not be nil")
	}
	// Verify cache TTL was set (we can't directly check, but we can verify cache works)
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec:       securityv1alpha1.ZenLockSpec{EncryptedData: map[string]string{"key": "value"}},
	}
	cacheKey := types.NamespacedName{Namespace: "default", Name: "test"}
	handler2.cache.Set(cacheKey, zenlock)
	if handler2.cache.Size() != 1 {
		t.Errorf("Expected cache size 1, got %d", handler2.cache.Size())
	}

	// Test with invalid cache TTL (should use default)
	os.Setenv("ZEN_LOCK_CACHE_TTL", "invalid-duration")
	handler3, err := NewPodHandler(client, scheme)
	if err != nil {
		t.Fatalf("Failed to create PodHandler with invalid TTL: %v", err)
	}
	if handler3.cache == nil {
		t.Error("PodHandler.cache should not be nil")
	}
}

func TestNewPodHandler_CacheTTL(t *testing.T) {
	originalKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("ZEN_LOCK_PRIVATE_KEY", originalKey)
		} else {
			os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")
		}
		os.Unsetenv("ZEN_LOCK_CACHE_TTL")
	}()

	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(securityv1alpha1.AddToScheme(scheme))

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	os.Setenv("ZEN_LOCK_PRIVATE_KEY", "AGE-SECRET-1EXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLE")

	// Test default TTL (5 minutes)
	handler1, err := NewPodHandler(client, scheme)
	if err != nil {
		t.Fatalf("Failed to create PodHandler: %v", err)
	}

	// Set a value and check it expires after default TTL
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec:       securityv1alpha1.ZenLockSpec{EncryptedData: map[string]string{"key": "value"}},
	}
	cacheKey := types.NamespacedName{Namespace: "default", Name: "test"}
	handler1.cache.Set(cacheKey, zenlock)

	// Create cache with short TTL for testing
	shortTTLCache := NewZenLockCache(50 * time.Millisecond)
	defer shortTTLCache.Stop()

	shortTTLCache.Set(cacheKey, zenlock)
	time.Sleep(100 * time.Millisecond)

	_, found := shortTTLCache.Get(cacheKey)
	if found {
		t.Error("Expected cache entry to expire")
	}
}
