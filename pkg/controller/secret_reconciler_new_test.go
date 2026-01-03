/*
Copyright 2025 Kube-ZEN Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"os"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1 "k8s.io/api/core/v1"
)

func TestNewSecretReconciler_DefaultOrphanTTL(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add corev1 to scheme: %v", err)
	}

	// Clear environment variable
	originalTTL := os.Getenv("ZEN_LOCK_ORPHAN_TTL")
	defer func() {
		if originalTTL != "" {
			os.Setenv("ZEN_LOCK_ORPHAN_TTL", originalTTL)
		} else {
			os.Unsetenv("ZEN_LOCK_ORPHAN_TTL")
		}
	}()
	os.Unsetenv("ZEN_LOCK_ORPHAN_TTL")

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := NewSecretReconciler(client, scheme)

	if reconciler.OrphanTTL != DefaultOrphanTTL {
		t.Errorf("Expected OrphanTTL %v, got %v", DefaultOrphanTTL, reconciler.OrphanTTL)
	}
}

func TestNewSecretReconciler_CustomOrphanTTL(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add corev1 to scheme: %v", err)
	}

	// Set custom TTL
	originalTTL := os.Getenv("ZEN_LOCK_ORPHAN_TTL")
	defer func() {
		if originalTTL != "" {
			os.Setenv("ZEN_LOCK_ORPHAN_TTL", originalTTL)
		} else {
			os.Unsetenv("ZEN_LOCK_ORPHAN_TTL")
		}
	}()
	os.Setenv("ZEN_LOCK_ORPHAN_TTL", "30m")

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := NewSecretReconciler(client, scheme)

	expectedTTL := 30 * time.Minute
	if reconciler.OrphanTTL != expectedTTL {
		t.Errorf("Expected OrphanTTL %v, got %v", expectedTTL, reconciler.OrphanTTL)
	}
}

func TestNewSecretReconciler_InvalidOrphanTTL(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add corev1 to scheme: %v", err)
	}

	// Set invalid TTL
	originalTTL := os.Getenv("ZEN_LOCK_ORPHAN_TTL")
	defer func() {
		if originalTTL != "" {
			os.Setenv("ZEN_LOCK_ORPHAN_TTL", originalTTL)
		} else {
			os.Unsetenv("ZEN_LOCK_ORPHAN_TTL")
		}
	}()
	os.Setenv("ZEN_LOCK_ORPHAN_TTL", "invalid-duration")

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := NewSecretReconciler(client, scheme)

	// Should fall back to default when parsing fails
	if reconciler.OrphanTTL != DefaultOrphanTTL {
		t.Errorf("Expected OrphanTTL to fall back to default %v, got %v", DefaultOrphanTTL, reconciler.OrphanTTL)
	}
}
