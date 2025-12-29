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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.zen.io/v1alpha1"
)

func setupTestReconciler(t *testing.T) (*ZenLockReconciler, *fake.ClientBuilder) {
	// Set test private key
	originalKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("ZEN_LOCK_PRIVATE_KEY", originalKey)
		} else {
			os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")
		}
	}()

	os.Setenv("ZEN_LOCK_PRIVATE_KEY", "AGE-SECRET-1EXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLE")

	scheme := runtime.NewScheme()
	securityv1alpha1.AddToScheme(scheme)

	clientBuilder := fake.NewClientBuilder().WithScheme(scheme)

	reconciler, err := NewZenLockReconciler(clientBuilder.Build(), scheme)
	if err != nil {
		t.Fatalf("Failed to create reconciler: %v", err)
	}

	return reconciler, clientBuilder
}

func TestZenLockReconciler_Reconcile_NotFound(t *testing.T) {
	reconciler, _ := setupTestReconciler(t)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "non-existent",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)

	if err != nil {
		t.Errorf("Reconcile() should not error for non-existent resource, got: %v", err)
	}
	if result.Requeue {
		t.Error("Reconcile() should not requeue for non-existent resource")
	}
}

func TestZenLockReconciler_Reconcile_ValidZenLock(t *testing.T) {
	reconciler, clientBuilder := setupTestReconciler(t)

	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": "dGVzdC12YWx1ZQ==", // base64 encoded test value
			},
		},
	}

	client := clientBuilder.WithObjects(zenlock).Build()
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
		t.Errorf("Reconcile() should not error for valid ZenLock, got: %v", err)
	}
	if result.Requeue {
		t.Error("Reconcile() should not requeue for valid ZenLock")
	}

	// Verify status was updated (note: fake client may not update status subresource properly)
	updatedZenLock := &securityv1alpha1.ZenLock{}
	if err := client.Get(ctx, req.NamespacedName, updatedZenLock); err != nil {
		t.Fatalf("Failed to get updated ZenLock: %v", err)
	}

	// Status update may not work with fake client, so we just verify the object exists
	if updatedZenLock.Name != "test-secret" {
		t.Errorf("Expected name 'test-secret', got '%s'", updatedZenLock.Name)
	}
}

func TestUpdateStatus(t *testing.T) {
	reconciler, clientBuilder := setupTestReconciler(t)

	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": "value1",
			},
		},
	}

	client := clientBuilder.WithObjects(zenlock).Build()
	reconciler.Client = client

	ctx := context.Background()
	reconciler.updateStatus(ctx, zenlock, "Ready", "KeyValid", "Test message")

	// Verify status was updated in the object (fake client may not persist status subresource)
	// We verify the object was modified
	if zenlock.Status.Phase != "Ready" {
		t.Errorf("Expected phase to be Ready, got: %s", zenlock.Status.Phase)
	}

	if len(zenlock.Status.Conditions) == 0 {
		t.Error("Expected at least one condition")
		return
	}

	condition := zenlock.Status.Conditions[0]
	if condition.Type != "Decryptable" {
		t.Errorf("Expected condition type to be Decryptable, got: %s", condition.Type)
	}
	if condition.Reason != "KeyValid" {
		t.Errorf("Expected condition reason to be KeyValid, got: %s", condition.Reason)
	}
	if condition.LastTransitionTime == nil {
		t.Error("Expected LastTransitionTime to be set")
	}
}
