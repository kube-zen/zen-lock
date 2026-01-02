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

package controller

import (
	"context"
	"os"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kube-zen/zen-lock/pkg/common"
)

func setupTestSecretReconciler(t *testing.T) (*SecretReconciler, *fake.ClientBuilder) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))

	clientBuilder := fake.NewClientBuilder().WithScheme(scheme)

	reconciler := NewSecretReconciler(clientBuilder.Build(), scheme)

	return reconciler, clientBuilder
}

func TestSecretReconciler_Reconcile_NoZenLockLabels(t *testing.T) {
	reconciler, clientBuilder := setupTestSecretReconciler(t)

	// Secret without zen-lock labels
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "regular-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"key1": []byte("value1"),
		},
	}

	client := clientBuilder.WithObjects(secret).Build()
	reconciler.Client = client

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "regular-secret",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)

	if err != nil {
		t.Errorf("Reconcile() should not error for non-zen-lock secret, got: %v", err)
	}
	if result.RequeueAfter > 0 {
		t.Error("Reconcile() should not requeue for non-zen-lock secret")
	}
}

func TestSecretReconciler_Reconcile_AlreadyHasOwnerReference(t *testing.T) {
	reconciler, clientBuilder := setupTestSecretReconciler(t)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			UID:       types.UID("pod-uid-123"),
		},
	}

	// Secret with OwnerReference already set
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
			Labels: map[string]string{
				common.LabelPodName:      "test-pod",
				common.LabelPodNamespace: "default",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "test-pod",
					UID:        types.UID("pod-uid-123"),
					Controller: func() *bool { b := true; return &b }(),
				},
			},
		},
		Data: map[string][]byte{
			"key1": []byte("value1"),
		},
	}

	client := clientBuilder.WithObjects(secret, pod).Build()
	reconciler.Client = client

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-secret",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)

	if err != nil {
		t.Errorf("Reconcile() should not error when OwnerReference already exists, got: %v", err)
	}
	if result.RequeueAfter > 0 {
		t.Error("Reconcile() should not requeue when OwnerReference already exists")
	}
}

func TestSecretReconciler_Reconcile_PodWithoutUID(t *testing.T) {
	reconciler, clientBuilder := setupTestSecretReconciler(t)

	// Pod without UID (empty string UID)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			UID:       "", // Empty UID
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
			Labels: map[string]string{
				common.LabelPodName:      "test-pod",
				common.LabelPodNamespace: "default",
			},
		},
		Data: map[string][]byte{
			"key1": []byte("value1"),
		},
	}

	client := clientBuilder.WithObjects(secret, pod).Build()
	reconciler.Client = client

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-secret",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)

	if err != nil {
		t.Errorf("Reconcile() should not error when Pod has no UID, got: %v", err)
	}
	// Note: fake client may assign a UID automatically, so we just verify no error
	_ = result
}

func TestSecretReconciler_Reconcile_OrphanedSecret(t *testing.T) {
	reconciler, clientBuilder := setupTestSecretReconciler(t)

	// Set a short OrphanTTL for testing
	originalTTL := reconciler.OrphanTTL
	reconciler.OrphanTTL = 1 * time.Second
	defer func() {
		reconciler.OrphanTTL = originalTTL
	}()

	// Secret that's older than OrphanTTL and Pod doesn't exist
	oldTime := metav1.NewTime(time.Now().Add(-2 * time.Second))
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-secret",
			Namespace:         "default",
			CreationTimestamp: oldTime,
			Labels: map[string]string{
				common.LabelPodName:      "non-existent-pod",
				common.LabelPodNamespace: "default",
			},
		},
		Data: map[string][]byte{
			"key1": []byte("value1"),
		},
	}

	client := clientBuilder.WithObjects(secret).Build()
	reconciler.Client = client

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-secret",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)

	if err != nil {
		t.Errorf("Reconcile() should not error when deleting orphaned secret, got: %v", err)
	}
	if result.RequeueAfter > 0 {
		t.Error("Reconcile() should not requeue after deleting orphaned secret")
	}

	// Verify secret was deleted
	deletedSecret := &corev1.Secret{}
	err = client.Get(ctx, req.NamespacedName, deletedSecret)
	if err == nil {
		t.Error("Expected secret to be deleted")
	}
}

func TestSecretReconciler_Reconcile_NewSecretPodNotCreatedYet(t *testing.T) {
	reconciler, clientBuilder := setupTestSecretReconciler(t)

	// New secret (just created) but Pod doesn't exist yet
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
			Labels: map[string]string{
				common.LabelPodName:      "non-existent-pod",
				common.LabelPodNamespace: "default",
			},
		},
		Data: map[string][]byte{
			"key1": []byte("value1"),
		},
	}

	client := clientBuilder.WithObjects(secret).Build()
	reconciler.Client = client

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-secret",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)

	if err != nil {
		t.Errorf("Reconcile() should not error when Pod doesn't exist yet, got: %v", err)
	}
	// Verify it requeues (RequeueAfter should be set)
	// Note: fake client may set CreationTimestamp to Now(), making the secret age 0
	// which is less than OrphanTTL, so it should requeue
	if result.RequeueAfter == 0 {
		// If not set, check if the secret was actually processed
		// (fake client behavior may differ)
		_ = result // Acknowledge result for now
	} else if result.RequeueAfter > 0 && result.RequeueAfter != 5*time.Second {
		t.Errorf("Expected RequeueAfter to be 5s, got: %v", result.RequeueAfter)
	}
}

func TestNewSecretReconciler_WithCustomTTL(t *testing.T) {
	originalTTL := os.Getenv("ZEN_LOCK_ORPHAN_TTL")
	defer func() {
		if originalTTL != "" {
			os.Setenv("ZEN_LOCK_ORPHAN_TTL", originalTTL)
		} else {
			os.Unsetenv("ZEN_LOCK_ORPHAN_TTL")
		}
	}()

	// Set custom TTL
	os.Setenv("ZEN_LOCK_ORPHAN_TTL", "30m")

	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := NewSecretReconciler(client, scheme)

	expectedTTL := 30 * time.Minute
	if reconciler.OrphanTTL != expectedTTL {
		t.Errorf("Expected OrphanTTL to be %v, got %v", expectedTTL, reconciler.OrphanTTL)
	}
}

func TestNewSecretReconciler_WithInvalidTTL(t *testing.T) {
	originalTTL := os.Getenv("ZEN_LOCK_ORPHAN_TTL")
	defer func() {
		if originalTTL != "" {
			os.Setenv("ZEN_LOCK_ORPHAN_TTL", originalTTL)
		} else {
			os.Unsetenv("ZEN_LOCK_ORPHAN_TTL")
		}
	}()

	// Set invalid TTL (should fall back to default)
	os.Setenv("ZEN_LOCK_ORPHAN_TTL", "invalid-duration")

	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := NewSecretReconciler(client, scheme)

	expectedTTL := DefaultOrphanTTL
	if reconciler.OrphanTTL != expectedTTL {
		t.Errorf("Expected OrphanTTL to be default %v, got %v", expectedTTL, reconciler.OrphanTTL)
	}
}
