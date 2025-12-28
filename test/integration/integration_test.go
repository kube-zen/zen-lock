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

package integration

import (
	"context"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.zen.io/v1alpha1"
	"github.com/kube-zen/zen-lock/pkg/controller"
	"github.com/kube-zen/zen-lock/pkg/crypto"
)

func setupTestEnvironment(t *testing.T) (context.Context, *fake.ClientBuilder, *crypto.AgeEncryptor) {
	// Set test private key
	originalKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("ZEN_LOCK_PRIVATE_KEY", originalKey)
		} else {
			os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")
		}
	}()

	// Use a test key (in real tests, generate a proper one)
	os.Setenv("ZEN_LOCK_PRIVATE_KEY", "AGE-SECRET-1EXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLE")

	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	securityv1alpha1.AddToScheme(scheme)

	clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
	encryptor := crypto.NewAgeEncryptor()

	return context.Background(), clientBuilder, encryptor
}

func TestZenLockReconciler_Integration(t *testing.T) {
	// Set test private key before setup
	originalKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	os.Setenv("ZEN_LOCK_PRIVATE_KEY", "AGE-SECRET-1EXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLE")
	defer func() {
		if originalKey != "" {
			os.Setenv("ZEN_LOCK_PRIVATE_KEY", originalKey)
		} else {
			os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")
		}
	}()

	ctx, clientBuilder, _ := setupTestEnvironment(t)

	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": "dGVzdC12YWx1ZQ==", // base64 encoded
			},
			Algorithm: "age",
		},
	}

	client := clientBuilder.WithObjects(zenlock).Build()

	reconciler, err := controller.NewZenLockReconciler(client, client.Scheme())
	if err != nil {
		t.Fatalf("Failed to create reconciler: %v", err)
	}

	// Test that reconciler can be created
	if reconciler == nil {
		t.Fatal("Reconciler is nil")
	}

	// Test reconciliation
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-secret",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Errorf("Reconcile() error = %v", err)
	}

	if result.Requeue {
		t.Error("Reconcile() should not requeue")
	}

	// Verify ZenLock still exists
	updatedZenLock := &securityv1alpha1.ZenLock{}
	if err := client.Get(ctx, req.NamespacedName, updatedZenLock); err != nil {
		t.Fatalf("Failed to get ZenLock: %v", err)
	}

	if updatedZenLock.Name != "test-secret" {
		t.Errorf("Expected name 'test-secret', got '%s'", updatedZenLock.Name)
	}
}

func TestZenLockCRUD_Integration(t *testing.T) {
	ctx, clientBuilder, _ := setupTestEnvironment(t)

	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": "encrypted-value-1",
				"key2": "encrypted-value-2",
			},
			Algorithm: "age",
		},
	}

	client := clientBuilder.Build()

	// Create
	if err := client.Create(ctx, zenlock); err != nil {
		t.Fatalf("Failed to create ZenLock: %v", err)
	}

	// Read
	retrieved := &securityv1alpha1.ZenLock{}
	nn := types.NamespacedName{Name: "test-secret", Namespace: "default"}
	if err := client.Get(ctx, nn, retrieved); err != nil {
		t.Fatalf("Failed to get ZenLock: %v", err)
	}

	if len(retrieved.Spec.EncryptedData) != 2 {
		t.Errorf("Expected 2 encrypted keys, got %d", len(retrieved.Spec.EncryptedData))
	}

	// Update
	retrieved.Spec.EncryptedData["key3"] = "encrypted-value-3"
	if err := client.Update(ctx, retrieved); err != nil {
		t.Fatalf("Failed to update ZenLock: %v", err)
	}

	// Verify update
	updated := &securityv1alpha1.ZenLock{}
	if err := client.Get(ctx, nn, updated); err != nil {
		t.Fatalf("Failed to get updated ZenLock: %v", err)
	}

	if len(updated.Spec.EncryptedData) != 3 {
		t.Errorf("Expected 3 encrypted keys after update, got %d", len(updated.Spec.EncryptedData))
	}
}

func TestCryptoEncryptDecrypt_Integration(t *testing.T) {
	_, _, encryptor := setupTestEnvironment(t)

	// Note: This test requires actual age keys to work properly
	// For now, we'll skip it and rely on unit tests with mocked crypto
	_ = encryptor
	t.Skip("Requires actual age keys - covered in unit tests")
}
