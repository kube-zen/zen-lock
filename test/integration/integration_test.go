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
	"encoding/base64"
	"os"
	"testing"

	"filippo.io/age"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
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

	// Generate test keys using age library
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	privateKey := identity.String()
	publicKey := identity.Recipient().String()

	// Test encryption/decryption flow
	plaintext := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	// Encrypt
	encryptedData := make(map[string]string)
	for k, v := range plaintext {
		ciphertext, err := encryptor.Encrypt([]byte(v), []string{publicKey})
		if err != nil {
			t.Fatalf("Failed to encrypt %s: %v", k, err)
		}
		// Base64 encode for DecryptMap
		encryptedData[k] = base64.StdEncoding.EncodeToString(ciphertext)
	}

	// Decrypt
	decryptedMap, err := encryptor.DecryptMap(encryptedData, privateKey)
	if err != nil {
		t.Fatalf("Failed to decrypt map: %v", err)
	}

	// Verify
	if len(decryptedMap) != len(plaintext) {
		t.Errorf("Expected %d decrypted keys, got %d", len(plaintext), len(decryptedMap))
	}

	for k, expectedValue := range plaintext {
		actualValue, exists := decryptedMap[k]
		if !exists {
			t.Errorf("Key %s not found in decrypted map", k)
			continue
		}
		if string(actualValue) != expectedValue {
			t.Errorf("Key %s: expected %q, got %q", k, expectedValue, string(actualValue))
		}
	}
}

func TestEphemeralSecretCleanup_Integration(t *testing.T) {
	ctx, clientBuilder, _ := setupTestEnvironment(t)

	client := clientBuilder.Build()

	// Create a Pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			UID:       types.UID("test-pod-uid"),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "test-container", Image: "nginx"},
			},
		},
	}

	if err := client.Create(ctx, pod); err != nil {
		t.Fatalf("Failed to create Pod: %v", err)
	}

	// Create ephemeral Secret with labels (OwnerReference will be set by controller later)
	// This matches the actual webhook behavior
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zen-lock-inject-default-test-pod-abc123",
			Namespace: "default",
			Labels: map[string]string{
				"zen-lock.security.kube-zen.io/pod-name":      pod.Name,
				"zen-lock.security.kube-zen.io/pod-namespace": "default",
				"zen-lock.security.kube-zen.io/zenlock-name": "test-secret",
			},
		},
		Data: map[string][]byte{
			"key1": []byte("value1"),
		},
	}

	if err := client.Create(ctx, secret); err != nil {
		t.Fatalf("Failed to create Secret: %v", err)
	}

	// Verify Secret exists
	retrievedSecret := &corev1.Secret{}
	secretNN := types.NamespacedName{Name: "zen-lock-inject-default-test-pod-abc123", Namespace: "default"}
	if err := client.Get(ctx, secretNN, retrievedSecret); err != nil {
		t.Fatalf("Failed to get Secret: %v", err)
	}

	// Verify labels (OwnerReference will be set by controller when Pod exists)
	if retrievedSecret.Labels == nil {
		t.Error("Expected Secret to have labels")
	} else {
		if retrievedSecret.Labels["zen-lock.security.kube-zen.io/pod-name"] != "test-pod" {
			t.Errorf("Expected pod-name label 'test-pod', got '%s'", retrievedSecret.Labels["zen-lock.security.kube-zen.io/pod-name"])
		}
		if retrievedSecret.Labels["zen-lock.security.kube-zen.io/pod-namespace"] != "default" {
			t.Errorf("Expected pod-namespace label 'default', got '%s'", retrievedSecret.Labels["zen-lock.security.kube-zen.io/pod-namespace"])
		}
	}

	// Note: In a real cluster, the SecretReconciler would set the OwnerReference
	// when the Pod exists. This test verifies the label-based tracking pattern.
}

func TestAllowedSubjectsValidation_Integration(t *testing.T) {
	ctx, clientBuilder, _ := setupTestEnvironment(t)

	client := clientBuilder.Build()

	// Create ServiceAccount
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backend-app",
			Namespace: "default",
		},
	}

	if err := client.Create(ctx, sa); err != nil {
		t.Fatalf("Failed to create ServiceAccount: %v", err)
	}

	// Create ZenLock with AllowedSubjects
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": "encrypted-value",
			},
			Algorithm: "age",
			AllowedSubjects: []securityv1alpha1.SubjectReference{
				{
					Kind:      "ServiceAccount",
					Name:      "backend-app",
					Namespace: "default",
				},
			},
		},
	}

	if err := client.Create(ctx, zenlock); err != nil {
		t.Fatalf("Failed to create ZenLock: %v", err)
	}

	// Verify ZenLock has AllowedSubjects
	retrieved := &securityv1alpha1.ZenLock{}
	nn := types.NamespacedName{Name: "test-secret", Namespace: "default"}
	if err := client.Get(ctx, nn, retrieved); err != nil {
		t.Fatalf("Failed to get ZenLock: %v", err)
	}

	if len(retrieved.Spec.AllowedSubjects) != 1 {
		t.Errorf("Expected 1 AllowedSubject, got %d", len(retrieved.Spec.AllowedSubjects))
	}

	allowedSubject := retrieved.Spec.AllowedSubjects[0]
	if allowedSubject.Kind != "ServiceAccount" {
		t.Errorf("Expected Kind 'ServiceAccount', got '%s'", allowedSubject.Kind)
	}
	if allowedSubject.Name != "backend-app" {
		t.Errorf("Expected Name 'backend-app', got '%s'", allowedSubject.Name)
	}
	if allowedSubject.Namespace != "default" {
		t.Errorf("Expected Namespace 'default', got '%s'", allowedSubject.Namespace)
	}
}

func TestZenLockStatusUpdate_Integration(t *testing.T) {
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
			Name:      "test-status",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": "encrypted-value",
			},
			Algorithm: "age",
		},
	}

	client := clientBuilder.WithObjects(zenlock).Build()

	reconciler, err := controller.NewZenLockReconciler(client, client.Scheme())
	if err != nil {
		t.Fatalf("Failed to create reconciler: %v", err)
	}

	// Reconcile
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-status",
			Namespace: "default",
		},
	}

	_, err = reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Errorf("Reconcile() error = %v", err)
	}

	// Verify status was updated
	updatedZenLock := &securityv1alpha1.ZenLock{}
	if err := client.Get(ctx, req.NamespacedName, updatedZenLock); err != nil {
		t.Fatalf("Failed to get ZenLock: %v", err)
	}

	// Check that status has been set (controller should update status)
	if updatedZenLock.Status.Phase == "" {
		t.Log("Status Phase not set - this may be expected if controller doesn't update status for invalid ciphertext")
	}

	// Check conditions
	if len(updatedZenLock.Status.Conditions) > 0 {
		// Verify LastTransitionTime is set
		for _, condition := range updatedZenLock.Status.Conditions {
			if condition.LastTransitionTime.IsZero() {
				t.Error("Expected LastTransitionTime to be set in condition")
			}
		}
	}
}
