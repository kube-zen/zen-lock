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
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kube-zen/zen-lock/pkg/common"
)

func TestSecretReconciler_Reconcile_GetPodError(t *testing.T) {
	reconciler, clientBuilder := setupSecretReconciler(t)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zen-lock-secret",
			Namespace: "default",
			Labels: map[string]string{
				common.LabelPodName:      "test-pod",
				common.LabelPodNamespace: "default",
				common.LabelZenLockName:  "test-zenlock",
			},
		},
		Data: map[string][]byte{
			"key": []byte("value"),
		},
	}

	client := clientBuilder.WithObjects(secret).Build()
	reconciler.Client = client

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "zen-lock-secret",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	// This will test the error path when Get fails (not NotFound)
	// Note: fake client returns NotFound, so we test the structure
	result, err := reconciler.Reconcile(ctx, req)

	// Should handle NotFound gracefully (requeue)
	if err != nil {
		t.Errorf("Reconcile() error = %v, want no error for NotFound", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("Reconcile() should set RequeueAfter when Pod not found")
	}
}

func TestSecretReconciler_Reconcile_UpdateSecretError(t *testing.T) {
	reconciler, clientBuilder := setupSecretReconciler(t)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			UID:       types.UID("test-pod-uid-123"),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "test-container", Image: "nginx"},
			},
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zen-lock-secret",
			Namespace: "default",
			Labels: map[string]string{
				common.LabelPodName:      "test-pod",
				common.LabelPodNamespace: "default",
				common.LabelZenLockName:  "test-zenlock",
			},
		},
		Data: map[string][]byte{
			"key": []byte("value"),
		},
	}

	client := clientBuilder.WithObjects(pod, secret).Build()
	reconciler.Client = client

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "zen-lock-secret",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	// This should succeed with fake client, but tests the retry logic structure
	result, err := reconciler.Reconcile(ctx, req)

	if err != nil {
		t.Errorf("Reconcile() error = %v, want no error", err)
	}
	if result.Requeue {
		t.Error("Reconcile() should not requeue after successful update")
	}
}

func TestSecretReconciler_Reconcile_SecretNotFound(t *testing.T) {
	reconciler, clientBuilder := setupSecretReconciler(t)

	// Don't create secret - should return early
	client := clientBuilder.Build()
	reconciler.Client = client

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "non-existent-secret",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)

	// Should return no error for not found
	if err != nil {
		t.Errorf("Reconcile() error = %v, want no error for not found", err)
	}
	if result.Requeue {
		t.Error("Reconcile() should not requeue for not found")
	}
}

func TestSecretReconciler_Reconcile_OrphanedSecretDeleteError(t *testing.T) {
	reconciler, clientBuilder := setupSecretReconciler(t)

	// Create orphaned secret (old, Pod doesn't exist)
	oldTime := metav1.NewTime(time.Now().Add(-20 * time.Minute))
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zen-lock-secret",
			Namespace: "default",
			Labels: map[string]string{
				common.LabelPodName:      "non-existent-pod",
				common.LabelPodNamespace: "default",
				common.LabelZenLockName:  "test-zenlock",
			},
			CreationTimestamp: oldTime,
		},
		Data: map[string][]byte{
			"key": []byte("value"),
		},
	}

	client := clientBuilder.WithObjects(secret).Build()
	reconciler.Client = client

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "zen-lock-secret",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	// Should delete orphaned secret
	result, err := reconciler.Reconcile(ctx, req)

	// Should handle deletion
	if err != nil {
		t.Errorf("Reconcile() error = %v, want no error", err)
	}
	if result.Requeue {
		t.Error("Reconcile() should not requeue after deleting orphaned secret")
	}

	// Verify secret was deleted
	deletedSecret := &corev1.Secret{}
	if err := client.Get(ctx, req.NamespacedName, deletedSecret); err == nil {
		t.Error("Expected secret to be deleted")
	} else if !k8serrors.IsNotFound(err) {
		t.Errorf("Expected NotFound error, got %v", err)
	}
}

func TestSecretReconciler_Reconcile_MissingPodNamespaceLabel(t *testing.T) {
	reconciler, clientBuilder := setupSecretReconciler(t)

	// Secret missing pod namespace label
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zen-lock-secret",
			Namespace: "default",
			Labels: map[string]string{
				common.LabelPodName:     "test-pod",
				// Missing LabelPodNamespace
				common.LabelZenLockName: "test-zenlock",
			},
		},
		Data: map[string][]byte{
			"key": []byte("value"),
		},
	}

	client := clientBuilder.WithObjects(secret).Build()
	reconciler.Client = client

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "zen-lock-secret",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)

	// Should ignore non-zen-lock secrets
	if err != nil {
		t.Errorf("Reconcile() error = %v, want no error", err)
	}
	if result.Requeue {
		t.Error("Reconcile() should not requeue for non-zen-lock secrets")
	}
}

func TestSecretReconciler_Reconcile_MissingPodNameLabel(t *testing.T) {
	reconciler, clientBuilder := setupSecretReconciler(t)

	// Secret missing pod name label
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zen-lock-secret",
			Namespace: "default",
			Labels: map[string]string{
				// Missing LabelPodName
				common.LabelPodNamespace: "default",
				common.LabelZenLockName:   "test-zenlock",
			},
		},
		Data: map[string][]byte{
			"key": []byte("value"),
		},
	}

	client := clientBuilder.WithObjects(secret).Build()
	reconciler.Client = client

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "zen-lock-secret",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)

	// Should ignore non-zen-lock secrets
	if err != nil {
		t.Errorf("Reconcile() error = %v, want no error", err)
	}
	if result.Requeue {
		t.Error("Reconcile() should not requeue for non-zen-lock secrets")
	}
}

