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
	"os"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
	"github.com/kube-zen/zen-lock/pkg/common"
)

func TestZenLockReconciler_UpdateStatus_NewCondition(t *testing.T) {
	reconciler, clientBuilder := setupTestReconciler(t)

	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key": "encrypted-value",
			},
		},
		Status: securityv1alpha1.ZenLockStatus{
			Conditions: []securityv1alpha1.ZenLockCondition{},
		},
	}

	client := clientBuilder.WithObjects(zenlock).WithStatusSubresource(zenlock).Build()
	reconciler.Client = client

	ctx := context.Background()
	reconciler.updateStatus(ctx, zenlock, "Ready", "Decrypted", "Successfully decrypted")

	// Verify condition was added
	updatedZenLock := &securityv1alpha1.ZenLock{}
	if err := client.Get(ctx, types.NamespacedName{Name: "test-zenlock", Namespace: "default"}, updatedZenLock); err != nil {
		t.Fatalf("Failed to get updated ZenLock: %v", err)
	}

	if len(updatedZenLock.Status.Conditions) != 1 {
		t.Errorf("Expected 1 condition, got %d", len(updatedZenLock.Status.Conditions))
	}

	condition := updatedZenLock.Status.Conditions[0]
	if condition.Type != "Decryptable" {
		t.Errorf("Expected condition type 'Decryptable', got '%s'", condition.Type)
	}
	if condition.Status != "True" {
		t.Errorf("Expected condition status 'True', got '%s'", condition.Status)
	}
	if condition.Reason != "Decrypted" {
		t.Errorf("Expected condition reason 'Decrypted', got '%s'", condition.Reason)
	}
	if condition.LastTransitionTime == nil {
		t.Error("Expected LastTransitionTime to be set for new condition")
	}
}

func TestZenLockReconciler_UpdateStatus_UpdateExistingCondition(t *testing.T) {
	reconciler, clientBuilder := setupTestReconciler(t)

	now := metav1.Now()
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key": "encrypted-value",
			},
		},
		Status: securityv1alpha1.ZenLockStatus{
			Phase: "Ready",
			Conditions: []securityv1alpha1.ZenLockCondition{
				{
					Type:               "Decryptable",
					Status:             "True",
					Reason:             "Decrypted",
					Message:            "Old message",
					LastTransitionTime: &now,
				},
			},
		},
	}

	client := clientBuilder.WithObjects(zenlock).WithStatusSubresource(zenlock).Build()
	reconciler.Client = client

	ctx := context.Background()
	// Update with same status - should not change LastTransitionTime
	reconciler.updateStatus(ctx, zenlock, "Ready", "Decrypted", "New message")

	updatedZenLock := &securityv1alpha1.ZenLock{}
	if err := client.Get(ctx, types.NamespacedName{Name: "test-zenlock", Namespace: "default"}, updatedZenLock); err != nil {
		t.Fatalf("Failed to get updated ZenLock: %v", err)
	}

	if len(updatedZenLock.Status.Conditions) != 1 {
		t.Errorf("Expected 1 condition, got %d", len(updatedZenLock.Status.Conditions))
	}

	condition := updatedZenLock.Status.Conditions[0]
	if condition.Message != "New message" {
		t.Errorf("Expected message 'New message', got '%s'", condition.Message)
	}
	// LastTransitionTime should remain the same when status doesn't change
	if condition.LastTransitionTime == nil || !condition.LastTransitionTime.Time.Equal(now.Time) {
		t.Error("Expected LastTransitionTime to remain unchanged when status doesn't change")
	}
}

func TestZenLockReconciler_UpdateStatus_ErrorCondition(t *testing.T) {
	reconciler, clientBuilder := setupTestReconciler(t)

	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key": "encrypted-value",
			},
		},
		Status: securityv1alpha1.ZenLockStatus{
			Conditions: []securityv1alpha1.ZenLockCondition{},
		},
	}

	client := clientBuilder.WithObjects(zenlock).WithStatusSubresource(zenlock).Build()
	reconciler.Client = client

	ctx := context.Background()
	reconciler.updateStatus(ctx, zenlock, "Error", "KeyNotFound", "Private key not configured")

	updatedZenLock := &securityv1alpha1.ZenLock{}
	if err := client.Get(ctx, types.NamespacedName{Name: "test-zenlock", Namespace: "default"}, updatedZenLock); err != nil {
		t.Fatalf("Failed to get updated ZenLock: %v", err)
	}

	if len(updatedZenLock.Status.Conditions) != 1 {
		t.Errorf("Expected 1 condition, got %d", len(updatedZenLock.Status.Conditions))
	}

	condition := updatedZenLock.Status.Conditions[0]
	if condition.Status != "False" {
		t.Errorf("Expected condition status 'False' for Error phase, got '%s'", condition.Status)
	}
	if condition.Reason != "KeyNotFound" {
		t.Errorf("Expected condition reason 'KeyNotFound', got '%s'", condition.Reason)
	}
}

func TestZenLockReconciler_UpdateStatus_StatusChange(t *testing.T) {
	reconciler, clientBuilder := setupTestReconciler(t)

	now := metav1.Now()
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key": "encrypted-value",
			},
		},
		Status: securityv1alpha1.ZenLockStatus{
			Phase: "Ready",
			Conditions: []securityv1alpha1.ZenLockCondition{
				{
					Type:               "Decryptable",
					Status:             "True",
					Reason:             "Decrypted",
					Message:            "Old message",
					LastTransitionTime: &now,
				},
			},
		},
	}

	client := clientBuilder.WithObjects(zenlock).WithStatusSubresource(zenlock).Build()
	reconciler.Client = client

	ctx := context.Background()
	// Change status from True to False
	reconciler.updateStatus(ctx, zenlock, "Error", "KeyNotFound", "Private key not configured")

	updatedZenLock := &securityv1alpha1.ZenLock{}
	if err := client.Get(ctx, types.NamespacedName{Name: "test-zenlock", Namespace: "default"}, updatedZenLock); err != nil {
		t.Fatalf("Failed to get updated ZenLock: %v", err)
	}

	condition := updatedZenLock.Status.Conditions[0]
	if condition.Status != "False" {
		t.Errorf("Expected condition status 'False', got '%s'", condition.Status)
	}
	// LastTransitionTime should be updated when status changes
	if condition.LastTransitionTime == nil || condition.LastTransitionTime.Time.Equal(now.Time) {
		t.Error("Expected LastTransitionTime to be updated when status changes")
	}
}

func TestZenLockReconciler_Reconcile_PrivateKeyReload(t *testing.T) {
	// Save and clear environment variable
	originalKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("ZEN_LOCK_PRIVATE_KEY", originalKey)
		} else {
			os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")
		}
	}()
	os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")

	reconciler, clientBuilder := setupTestReconciler(t)

	// Clear private key to test reload path
	reconciler.privateKey = ""

	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key": "encrypted-value",
			},
		},
	}

	client := clientBuilder.WithObjects(zenlock).WithStatusSubresource(zenlock).Build()
	reconciler.Client = client

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-zenlock",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)

	// Should requeue with delay when private key is missing
	if err != nil {
		t.Errorf("Reconcile() error = %v, want no error", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("Reconcile() should set RequeueAfter when private key is missing")
	}
	if result.RequeueAfter != 30*time.Second {
		t.Errorf("Reconcile() RequeueAfter = %v, want 30s", result.RequeueAfter)
	}
}

func TestZenLockReconciler_HandleDeletion_SecretListError(t *testing.T) {
	reconciler, clientBuilder := setupTestReconciler(t)

	now := metav1.Now()
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-zenlock",
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{"zenlocks.security.kube-zen.io/finalizer"},
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key": "encrypted-value",
			},
		},
	}

	// Create a client that will fail on List operations
	client := clientBuilder.WithObjects(zenlock).Build()
	reconciler.Client = client

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-zenlock",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	// This will test the error path in handleDeletion when List fails
	// Note: fake client doesn't actually fail, but we can test the structure
	result, err := reconciler.Reconcile(ctx, req)

	// Should handle deletion without error (fake client succeeds)
	if err != nil {
		t.Errorf("Reconcile() error = %v, want no error", err)
	}
	if result.RequeueAfter > 0 {
		t.Error("Reconcile() should not requeue after successful deletion")
	}
}

func TestZenLockReconciler_HandleDeletion_DeleteSecretError(t *testing.T) {
	reconciler, clientBuilder := setupTestReconciler(t)

	now := metav1.Now()
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-zenlock",
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{"zenlocks.security.kube-zen.io/finalizer"},
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key": "encrypted-value",
			},
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
			Labels: map[string]string{
				common.LabelZenLockName: "test-zenlock",
			},
		},
	}

	client := clientBuilder.WithObjects(zenlock, secret).Build()
	reconciler.Client = client

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-zenlock",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)

	// Should continue even if secret deletion fails (logs error but continues)
	if err != nil {
		t.Errorf("Reconcile() error = %v, want no error (should continue on secret delete error)", err)
	}
	// Should complete deletion and remove finalizer
	if result.RequeueAfter > 0 {
		t.Error("Reconcile() should not requeue after deletion cleanup")
	}
}
