//go:build e2e
// +build e2e

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

package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.zen.io/v1alpha1"
)

var (
	testEnv   *envtest.Environment
	k8sClient client.Client
)

func TestMain(m *testing.M) {
	// Setup test environment
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{"../../config/crd/bases"},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	if err != nil {
		panic(err)
	}

	// Create client
	k8sClient, err = client.New(cfg, client.Options{})
	if err != nil {
		panic(err)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	if err := testEnv.Stop(); err != nil {
		panic(err)
	}

	os.Exit(code)
}

func TestZenLockCRD_Exists(t *testing.T) {
	// Verify CRD exists
	zenlock := &securityv1alpha1.ZenLock{}
	gvk := zenlock.GroupVersionKind()
	if gvk.Group != "security.zen.io" {
		t.Errorf("Expected group 'security.zen.io', got '%s'", gvk.Group)
	}
	if gvk.Version != "v1alpha1" {
		t.Errorf("Expected version 'v1alpha1', got '%s'", gvk.Version)
	}
	if gvk.Kind != "ZenLock" {
		t.Errorf("Expected kind 'ZenLock', got '%s'", gvk.Kind)
	}
}

func TestZenLockCRUD_E2E(t *testing.T) {
	ctx := context.Background()
	namespace := "default"

	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "e2e-test-secret",
			Namespace: namespace,
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": "encrypted-value-1",
				"key2": "encrypted-value-2",
			},
			Algorithm: "age",
		},
	}

	// Create
	if err := k8sClient.Create(ctx, zenlock); err != nil {
		t.Fatalf("Failed to create ZenLock: %v", err)
	}
	defer func() {
		k8sClient.Delete(ctx, zenlock)
	}()

	// Read
	retrieved := &securityv1alpha1.ZenLock{}
	nn := types.NamespacedName{Name: "e2e-test-secret", Namespace: namespace}
	if err := k8sClient.Get(ctx, nn, retrieved); err != nil {
		t.Fatalf("Failed to get ZenLock: %v", err)
	}

	if len(retrieved.Spec.EncryptedData) != 2 {
		t.Errorf("Expected 2 encrypted keys, got %d", len(retrieved.Spec.EncryptedData))
	}

	// Update
	retrieved.Spec.EncryptedData["key3"] = "encrypted-value-3"
	if err := k8sClient.Update(ctx, retrieved); err != nil {
		t.Fatalf("Failed to update ZenLock: %v", err)
	}

	// Verify update
	updated := &securityv1alpha1.ZenLock{}
	if err := k8sClient.Get(ctx, nn, updated); err != nil {
		t.Fatalf("Failed to get updated ZenLock: %v", err)
	}

	if len(updated.Spec.EncryptedData) != 3 {
		t.Errorf("Expected 3 encrypted keys after update, got %d", len(updated.Spec.EncryptedData))
	}

	// Delete
	if err := k8sClient.Delete(ctx, updated); err != nil {
		t.Fatalf("Failed to delete ZenLock: %v", err)
	}

	// Verify deletion
	deleted := &securityv1alpha1.ZenLock{}
	if err := k8sClient.Get(ctx, nn, deleted); err == nil {
		t.Error("Expected ZenLock to be deleted")
	}
}

func TestPodInjection_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := context.Background()
	namespace := "default"

	// Create ZenLock
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "e2e-injection-test",
			Namespace: namespace,
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"TEST_KEY": "dGVzdC12YWx1ZQ==", // base64 encoded
			},
		},
	}

	if err := k8sClient.Create(ctx, zenlock); err != nil {
		t.Fatalf("Failed to create ZenLock: %v", err)
	}
	defer func() {
		k8sClient.Delete(ctx, zenlock)
	}()

	// Create Pod with injection annotation
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "e2e-test-pod",
			Namespace: namespace,
			Annotations: map[string]string{
				"zen-lock/inject": "e2e-injection-test",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "test-container",
					Image:   "busybox:latest",
					Command: []string{"sleep", "3600"},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	// Note: This test requires a running webhook server
	// In a real E2E test, you would:
	// 1. Deploy the webhook server
	// 2. Create the Pod
	// 3. Verify the secret was injected
	// 4. Verify the Pod can read the secret

	t.Skip("E2E test requires running webhook server - implement with test environment setup")
}

func TestAllowedSubjects_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := context.Background()
	namespace := "default"

	// Create ServiceAccount
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "e2e-test-sa",
			Namespace: namespace,
		},
	}

	if err := k8sClient.Create(ctx, sa); err != nil {
		t.Fatalf("Failed to create ServiceAccount: %v", err)
	}
	defer func() {
		k8sClient.Delete(ctx, sa)
	}()

	// Create ZenLock with AllowedSubjects
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "e2e-allowed-subjects-test",
			Namespace: namespace,
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": "value1",
			},
			AllowedSubjects: []securityv1alpha1.SubjectReference{
				{
					Kind:      "ServiceAccount",
					Name:      "e2e-test-sa",
					Namespace: namespace,
				},
			},
		},
	}

	if err := k8sClient.Create(ctx, zenlock); err != nil {
		t.Fatalf("Failed to create ZenLock: %v", err)
	}
	defer func() {
		k8sClient.Delete(ctx, zenlock)
	}()

	// Verify ZenLock was created with AllowedSubjects
	retrieved := &securityv1alpha1.ZenLock{}
	nn := types.NamespacedName{Name: "e2e-allowed-subjects-test", Namespace: namespace}
	if err := k8sClient.Get(ctx, nn, retrieved); err != nil {
		t.Fatalf("Failed to get ZenLock: %v", err)
	}

	if len(retrieved.Spec.AllowedSubjects) != 1 {
		t.Errorf("Expected 1 allowed subject, got %d", len(retrieved.Spec.AllowedSubjects))
	}

	if retrieved.Spec.AllowedSubjects[0].Name != "e2e-test-sa" {
		t.Errorf("Expected ServiceAccount name 'e2e-test-sa', got '%s'", retrieved.Spec.AllowedSubjects[0].Name)
	}

	t.Skip("E2E test requires running webhook server to test actual validation")
}

// Helper function to wait for resource
func waitForResource(ctx context.Context, client client.Client, obj client.Object, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := client.Get(ctx, client.ObjectKeyFromObject(obj), obj); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return context.DeadlineExceeded
}
