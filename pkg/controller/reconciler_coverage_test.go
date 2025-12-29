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
	"encoding/base64"
	"os"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"filippo.io/age"
	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
	"github.com/kube-zen/zen-lock/pkg/crypto"
)

func TestZenLockReconciler_Reconcile_SuccessfulDecryption(t *testing.T) {
	reconciler, clientBuilder := setupTestReconciler(t)

	// Generate real age keys for testing
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}
	privateKey := identity.String()
	publicKey := identity.Recipient().String()

	// Set private key
	originalKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	os.Setenv("ZEN_LOCK_PRIVATE_KEY", privateKey)
	defer func() {
		if originalKey != "" {
			os.Setenv("ZEN_LOCK_PRIVATE_KEY", originalKey)
		} else {
			os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")
		}
	}()

	// Encrypt some data
	encryptor := crypto.NewAgeEncryptor()
	plaintext := []byte("test-secret-value")
	ciphertext, err := encryptor.Encrypt(plaintext, []string{publicKey})
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}
	encryptedData := base64.StdEncoding.EncodeToString(ciphertext)

	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
			Finalizers: []string{zenLockFinalizer}, // Pre-add finalizer
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": encryptedData,
			},
		},
	}

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

	if err != nil {
		t.Errorf("Reconcile() should not error for valid encrypted data, got: %v", err)
	}
	if result.Requeue {
		t.Error("Reconcile() should not requeue after successful decryption")
	}

	// Note: fake client doesn't persist status updates, so we verify the reconcile completed successfully
	// The status update is tested separately in TestUpdateStatus
	_ = result
}

func TestZenLockReconciler_Reconcile_FinalizerAddition(t *testing.T) {
	reconciler, clientBuilder := setupTestReconciler(t)

	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
			// No finalizer - will be added on first reconcile
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": "dGVzdA==",
			},
		},
	}

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

	if err != nil {
		t.Errorf("Reconcile() should not error when adding finalizer, got: %v", err)
	}
	if !result.Requeue {
		t.Error("Reconcile() should requeue after adding finalizer")
	}

	// Verify finalizer was added
	updatedZenLock := &securityv1alpha1.ZenLock{}
	if err := client.Get(ctx, req.NamespacedName, updatedZenLock); err != nil {
		t.Fatalf("Failed to get updated ZenLock: %v", err)
	}

	if !containsString(updatedZenLock.Finalizers, zenLockFinalizer) {
		t.Error("Expected finalizer to be added")
	}
}

func TestUpdateStatus_ExistingCondition_Coverage(t *testing.T) {
	reconciler, clientBuilder := setupTestReconciler(t)

	now := metav1.Now()
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": "dGVzdA==",
			},
		},
		Status: securityv1alpha1.ZenLockStatus{
			Phase: "Error",
			Conditions: []securityv1alpha1.ZenLockCondition{
				{
					Type:               "Decryptable",
					Status:             "False",
					Reason:             "DecryptionFailed",
					Message:            "Previous error",
					LastTransitionTime: &now,
				},
			},
		},
	}

	client := clientBuilder.WithObjects(zenlock).Build()
	reconciler.Client = client

	ctx := context.Background()
	// Update status to Ready (status change should update LastTransitionTime)
	reconciler.updateStatus(ctx, zenlock, "Ready", "KeyValid", "Success")

	// Verify condition was updated
	if len(zenlock.Status.Conditions) != 1 {
		t.Errorf("Expected 1 condition, got %d", len(zenlock.Status.Conditions))
	}

	condition := zenlock.Status.Conditions[0]
	if condition.Status != "True" {
		t.Errorf("Expected condition status to be True, got: %s", condition.Status)
	}
	if condition.Reason != "KeyValid" {
		t.Errorf("Expected condition reason to be KeyValid, got: %s", condition.Reason)
	}
	// LastTransitionTime should be updated when status changes
	if condition.LastTransitionTime == nil {
		t.Error("Expected LastTransitionTime to be set")
	}
}

func TestUpdateStatus_NoStatusChange_Coverage(t *testing.T) {
	reconciler, clientBuilder := setupTestReconciler(t)

	now := metav1.Now()
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": "dGVzdA==",
			},
		},
		Status: securityv1alpha1.ZenLockStatus{
			Phase: "Ready",
			Conditions: []securityv1alpha1.ZenLockCondition{
				{
					Type:               "Decryptable",
					Status:             "True",
					Reason:             "KeyValid",
					Message:            "Previous success",
					LastTransitionTime: &now,
				},
			},
		},
	}

	client := clientBuilder.WithObjects(zenlock).Build()
	reconciler.Client = client

	ctx := context.Background()
	originalTransitionTime := zenlock.Status.Conditions[0].LastTransitionTime

	// Update status to Ready again (no status change)
	reconciler.updateStatus(ctx, zenlock, "Ready", "KeyValid", "Still valid")

	// Verify LastTransitionTime was preserved (not updated)
	if zenlock.Status.Conditions[0].LastTransitionTime != originalTransitionTime {
		t.Error("Expected LastTransitionTime to be preserved when status doesn't change")
	}
}

