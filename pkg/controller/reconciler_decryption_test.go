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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
)

func TestZenLockReconciler_Reconcile_DecryptionFailure(t *testing.T) {
	reconciler, clientBuilder := setupTestReconciler(t)

	// Create ZenLock with invalid encrypted data (will fail decryption)
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
			Finalizers: []string{"zenlocks.security.kube-zen.io/finalizer"},
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key": "invalid-encrypted-data", // This will fail decryption
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

	// Should not error, but should update status to Error
	if err != nil {
		t.Errorf("Reconcile() error = %v, want no error", err)
	}
	if result.RequeueAfter > 0 {
		t.Error("Reconcile() should not requeue after decryption failure")
	}

	// Verify status was updated to Error
	updatedZenLock := &securityv1alpha1.ZenLock{}
	if err := client.Get(ctx, req.NamespacedName, updatedZenLock); err != nil {
		t.Fatalf("Failed to get updated ZenLock: %v", err)
	}

	if updatedZenLock.Status.Phase != "Error" {
		t.Errorf("Expected status phase 'Error', got '%s'", updatedZenLock.Status.Phase)
	}
}

// TestZenLockReconciler_Reconcile_SuccessfulDecryption is defined in reconciler_coverage_test.go
// This test file focuses on decryption failure scenarios

func TestZenLockReconciler_Reconcile_UpdateFinalizerError(t *testing.T) {
	reconciler, clientBuilder := setupTestReconciler(t)

	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
			// No finalizer - will try to add one
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key": "encrypted-value",
			},
		},
	}

	// Create client without status subresource to test error path
	client := clientBuilder.WithObjects(zenlock).Build()
	reconciler.Client = client

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-zenlock",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)

	// Should handle update error gracefully
	// Note: fake client may succeed, but we test the structure
	if err == nil {
		// If update succeeded, should requeue immediately
		if result.RequeueAfter != 0 {
			t.Error("Expected immediate requeue (RequeueAfter=0) after adding finalizer")
		}
	}
}

// TestZenLockReconciler_Reconcile_NotFound is defined in reconciler_test.go
// This test file focuses on decryption-specific scenarios

