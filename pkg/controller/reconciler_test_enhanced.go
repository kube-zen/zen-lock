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

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
)

// setupTestReconcilerEnhanced is a helper function for enhanced tests
func setupTestReconcilerEnhanced(t *testing.T) (*ZenLockReconciler, *fake.ClientBuilder) {
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
	if err := securityv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add securityv1alpha1 to scheme: %v", err)
	}

	clientBuilder := fake.NewClientBuilder().WithScheme(scheme)

	reconciler, err := NewZenLockReconciler(clientBuilder.Build(), scheme)
	if err != nil {
		t.Fatalf("Failed to create reconciler: %v", err)
	}

	return reconciler, clientBuilder
}

func TestZenLockReconciler_Reconcile_NoPrivateKey(t *testing.T) {
	reconciler, clientBuilder := setupTestReconcilerEnhanced(t)
	if reconciler == nil {
		t.Fatal("setupTestReconciler returned nil reconciler")
	}

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

	// Unset private key to test error path
	originalKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("ZEN_LOCK_PRIVATE_KEY", originalKey)
		}
	}()

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-secret",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)

	if err != nil {
		t.Errorf("Reconcile() should not error when private key is missing (status updated instead), got: %v", err)
	}
	if result.RequeueAfter > 0 {
		t.Error("Reconcile() should not requeue when private key is missing")
	}

	// Verify status was updated to Error
	updatedZenLock := &securityv1alpha1.ZenLock{}
	if err := client.Get(ctx, req.NamespacedName, updatedZenLock); err != nil {
		t.Fatalf("Failed to get updated ZenLock: %v", err)
	}

	if updatedZenLock.Status.Phase != "Error" {
		t.Errorf("Expected phase to be Error, got: %s", updatedZenLock.Status.Phase)
	}
}

func TestZenLockReconciler_Reconcile_DecryptionFailed(t *testing.T) {
	reconciler, clientBuilder := setupTestReconcilerEnhanced(t)
	if reconciler == nil {
		t.Fatal("setupTestReconciler returned nil reconciler")
	}

	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": "invalid-ciphertext-not-base64",
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

	// Decryption should fail, but reconcile should not error (status updated instead)
	if err != nil {
		t.Errorf("Reconcile() should not error on decryption failure (status updated instead), got: %v", err)
	}
	if result.RequeueAfter > 0 {
		t.Error("Reconcile() should not requeue on decryption failure")
	}

	// Verify status was updated to Error
	updatedZenLock := &securityv1alpha1.ZenLock{}
	if err := client.Get(ctx, req.NamespacedName, updatedZenLock); err != nil {
		t.Fatalf("Failed to get updated ZenLock: %v", err)
	}

	if updatedZenLock.Status.Phase != "Error" {
		t.Errorf("Expected phase to be Error, got: %s", updatedZenLock.Status.Phase)
	}
}
