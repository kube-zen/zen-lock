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
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"testing"
	"time"

	"filippo.io/age"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.zen.io/v1alpha1"
	"github.com/kube-zen/zen-lock/pkg/controller"
	webhookpkg "github.com/kube-zen/zen-lock/pkg/webhook"
)

var (
	testEnv   *envtest.Environment
	k8sClient client.Client
	scheme    = runtime.NewScheme()
	testCtx   context.Context
	cancel    context.CancelFunc
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(securityv1alpha1.AddToScheme(scheme))
}

func TestMain(m *testing.M) {
	logf.SetLogger(zap.New(zap.UseDevMode(true)))

	// Set a default test private key for the controller (tests can override)
	if os.Getenv("ZEN_LOCK_PRIVATE_KEY") == "" {
		os.Setenv("ZEN_LOCK_PRIVATE_KEY", "AGE-SECRET-1EXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLE")
	}

	// Setup test environment
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{"../../config/crd/bases"},
		ErrorIfCRDPathMissing: true,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{"../../config/webhook"},
		},
	}

	cfg, err := testEnv.Start()
	if err != nil {
		panic(fmt.Sprintf("Failed to start test environment: %v", err))
	}

	// Create manager with webhook server
	testCtx, cancel = context.WithCancel(context.Background())
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0", // Disable metrics server for tests
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    testEnv.WebhookInstallOptions.LocalServingPort,
			Host:    testEnv.WebhookInstallOptions.LocalServingHost,
			CertDir: testEnv.WebhookInstallOptions.LocalServingCertDir,
		}),
		LeaderElection: false,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to create manager: %v", err))
	}

	// Setup controller
	reconciler, err := controller.NewZenLockReconciler(mgr.GetClient(), mgr.GetScheme())
	if err != nil {
		panic(fmt.Sprintf("Failed to create reconciler: %v", err))
	}
	if err := reconciler.SetupWithManager(mgr); err != nil {
		panic(fmt.Sprintf("Failed to setup controller: %v", err))
	}

	// Setup webhook
	if err := webhookpkg.SetupWebhookWithManager(mgr); err != nil {
		panic(fmt.Sprintf("Failed to setup webhook: %v", err))
	}

	// Start manager in background
	go func() {
		if err := mgr.Start(testCtx); err != nil {
			panic(fmt.Sprintf("Failed to start manager: %v", err))
		}
	}()

	// Wait for manager to be ready
	time.Sleep(2 * time.Second)

	// Create client
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		panic(fmt.Sprintf("Failed to create client: %v", err))
	}

	// Run tests
	code := m.Run()

	// Cleanup
	cancel()
	if err := testEnv.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop test environment: %v", err))
	}

	os.Exit(code)
}

// generateTestKeys generates a test key pair for E2E tests
func generateTestKeys(t *testing.T) (privateKey, publicKey string) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate test identity: %v", err)
	}
	return identity.String(), identity.Recipient().String()
}

// encryptTestData encrypts test data using the provided public key
func encryptTestData(t *testing.T, plaintext string, publicKey string) string {
	recipient, err := age.ParseX25519Recipient(publicKey)
	if err != nil {
		t.Fatalf("Failed to parse recipient: %v", err)
	}

	var encrypted bytes.Buffer
	w, err := age.Encrypt(&encrypted, recipient)
	if err != nil {
		t.Fatalf("Failed to create encrypt writer: %v", err)
	}

	if _, err := w.Write([]byte(plaintext)); err != nil {
		t.Fatalf("Failed to write plaintext: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close encrypt writer: %v", err)
	}

	return base64.StdEncoding.EncodeToString(encrypted.Bytes())
}

func TestZenLockCRD_Exists(t *testing.T) {
	// Verify CRD exists by checking the scheme
	zenlock := &securityv1alpha1.ZenLock{}
	kinds, _, _ := scheme.ObjectKinds(zenlock)
	if len(kinds) == 0 {
		t.Fatal("Expected at least one GVK for ZenLock")
	}
	gvk := kinds[0]
	if gvk.Group != "security.zen.io" {
		t.Errorf("Expected group 'security.zen.io', got '%s'", gvk.Group)
	}
	if gvk.Version != "v1alpha1" {
		t.Errorf("Expected version 'v1alpha1', got '%s'", gvk.Version)
	}
	if gvk.Kind != "ZenLock" {
		t.Errorf("Expected kind 'ZenLock', got '%s'", gvk.Kind)
	}
	
	// Also verify we can create a GVK directly
	expectedGVK := schema.GroupVersionKind{
		Group:   "security.zen.io",
		Version: "v1alpha1",
		Kind:    "ZenLock",
	}
	if gvk != expectedGVK {
		t.Errorf("Expected GVK %v, got %v", expectedGVK, gvk)
	}
}

func TestZenLockCRUD_E2E(t *testing.T) {
	ctx := context.Background()
	namespace := "default"

	// Create namespace with zen-lock label for webhook
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				"zen-lock": "enabled",
			},
		},
	}
	_ = k8sClient.Create(ctx, ns) // Ignore error if exists

	privateKey, publicKey := generateTestKeys(t)
	encryptedValue := encryptTestData(t, "test-value", publicKey)

	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "e2e-test-secret",
			Namespace: namespace,
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": encryptedValue,
			},
			Algorithm: "age",
		},
	}

	// Set private key for controller
	os.Setenv("ZEN_LOCK_PRIVATE_KEY", privateKey)
	defer os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")

	// Create
	if err := k8sClient.Create(ctx, zenlock); err != nil {
		t.Fatalf("Failed to create ZenLock: %v", err)
	}
	defer func() {
		k8sClient.Delete(ctx, zenlock)
	}()

	// Wait for controller to reconcile
	time.Sleep(2 * time.Second)

	// Read
	retrieved := &securityv1alpha1.ZenLock{}
	nn := types.NamespacedName{Name: "e2e-test-secret", Namespace: namespace}
	if err := k8sClient.Get(ctx, nn, retrieved); err != nil {
		t.Fatalf("Failed to get ZenLock: %v", err)
	}

	if len(retrieved.Spec.EncryptedData) != 1 {
		t.Errorf("Expected 1 encrypted key, got %d", len(retrieved.Spec.EncryptedData))
	}

	// Update
	encryptedValue2 := encryptTestData(t, "test-value-2", publicKey)
	retrieved.Spec.EncryptedData["key2"] = encryptedValue2
	if err := k8sClient.Update(ctx, retrieved); err != nil {
		t.Fatalf("Failed to update ZenLock: %v", err)
	}

	// Verify update
	updated := &securityv1alpha1.ZenLock{}
	if err := k8sClient.Get(ctx, nn, updated); err != nil {
		t.Fatalf("Failed to get updated ZenLock: %v", err)
	}

	if len(updated.Spec.EncryptedData) != 2 {
		t.Errorf("Expected 2 encrypted keys after update, got %d", len(updated.Spec.EncryptedData))
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

func TestZenLockController_Reconciliation(t *testing.T) {
	ctx := context.Background()
	namespace := "default"

	// Create namespace with zen-lock label for webhook
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				"zen-lock": "enabled",
			},
		},
	}
	_ = k8sClient.Create(ctx, ns) // Ignore error if exists

	privateKey, publicKey := generateTestKeys(t)
	encryptedValue := encryptTestData(t, "test-value", publicKey)

	// Set private key for controller BEFORE creating ZenLock
	originalKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	os.Setenv("ZEN_LOCK_PRIVATE_KEY", privateKey)
	defer func() {
		if originalKey != "" {
			os.Setenv("ZEN_LOCK_PRIVATE_KEY", originalKey)
		} else {
			os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")
		}
	}()

	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "e2e-reconcile-test",
			Namespace: namespace,
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"TEST_KEY": encryptedValue,
			},
			Algorithm: "age",
		},
	}

	// Create ZenLock
	if err := k8sClient.Create(ctx, zenlock); err != nil {
		t.Fatalf("Failed to create ZenLock: %v", err)
	}
	defer func() {
		k8sClient.Delete(ctx, zenlock)
	}()

	// Wait for controller to reconcile - retry until status is updated
	var retrieved *securityv1alpha1.ZenLock
	nn := types.NamespacedName{Name: "e2e-reconcile-test", Namespace: namespace}
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		retrieved = &securityv1alpha1.ZenLock{}
		if err := k8sClient.Get(ctx, nn, retrieved); err != nil {
			t.Fatalf("Failed to get ZenLock: %v", err)
		}
		if retrieved.Status.Phase != "" {
			break
		}
	}

	// Check that status has been updated
	if retrieved.Status.Phase == "" {
		t.Error("Expected Phase to be set by controller")
	}

	// Check conditions - controller sets "Decryptable" condition
	decryptableCondition := findCondition(retrieved.Status.Conditions, "Decryptable")
	if decryptableCondition == nil {
		t.Error("Expected Decryptable condition to be set")
	} else if decryptableCondition.Status != "True" {
		t.Errorf("Expected Decryptable condition to be True, got %s", decryptableCondition.Status)
	}
}

func TestPodInjection_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := context.Background()
	namespace := "default"

	// Create namespace with zen-lock label for webhook
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				"zen-lock": "enabled",
			},
		},
	}
	_ = k8sClient.Create(ctx, ns) // Ignore error if exists

	privateKey, publicKey := generateTestKeys(t)
	encryptedValue := encryptTestData(t, "test-secret-value", publicKey)

	// Set private key for webhook and controller
	os.Setenv("ZEN_LOCK_PRIVATE_KEY", privateKey)
	defer os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")

	// Create ZenLock
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "e2e-injection-test",
			Namespace: namespace,
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"TEST_KEY": encryptedValue,
			},
			Algorithm: "age",
		},
	}

	if err := k8sClient.Create(ctx, zenlock); err != nil {
		t.Fatalf("Failed to create ZenLock: %v", err)
	}
	defer func() {
		k8sClient.Delete(ctx, zenlock)
	}()

	// Wait for controller to reconcile
	time.Sleep(2 * time.Second)

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

	// Create Pod - webhook should mutate it
	if err := k8sClient.Create(ctx, pod); err != nil {
		t.Fatalf("Failed to create Pod: %v", err)
	}
	defer func() {
		k8sClient.Delete(ctx, pod)
	}()

	// Wait for webhook to process
	time.Sleep(2 * time.Second)

	// Verify Pod was mutated
	retrievedPod := &corev1.Pod{}
	nn := types.NamespacedName{Name: "e2e-test-pod", Namespace: namespace}
	if err := k8sClient.Get(ctx, nn, retrievedPod); err != nil {
		t.Fatalf("Failed to get Pod: %v", err)
	}

	// Check that volume was added
	foundVolume := false
	for _, vol := range retrievedPod.Spec.Volumes {
		if vol.Name == "zen-lock-e2e-injection-test" {
			foundVolume = true
			if vol.Secret == nil {
				t.Error("Expected volume to reference a Secret")
			} else if vol.Secret.SecretName != "zen-lock-e2e-injection-test" {
				t.Errorf("Expected secret name 'zen-lock-e2e-injection-test', got '%s'", vol.Secret.SecretName)
			}
			break
		}
	}
	if !foundVolume {
		t.Error("Expected Pod to have zen-lock volume injected")
	}

	// Check that volume mount was added
	foundMount := false
	for _, container := range retrievedPod.Spec.Containers {
		for _, mount := range container.VolumeMounts {
			if mount.Name == "zen-lock-e2e-injection-test" {
				foundMount = true
				if mount.MountPath != "/zen-lock/secrets" {
					t.Errorf("Expected mount path '/zen-lock/secrets', got '%s'", mount.MountPath)
				}
				break
			}
		}
	}
	if !foundMount {
		t.Error("Expected Pod to have zen-lock volume mount injected")
	}

	// Verify ephemeral Secret was created
	secret := &corev1.Secret{}
	secretNN := types.NamespacedName{Name: "zen-lock-e2e-injection-test", Namespace: namespace}
	if err := k8sClient.Get(ctx, secretNN, secret); err != nil {
		t.Fatalf("Failed to get ephemeral Secret: %v", err)
	}

	// Verify Secret has OwnerReference to Pod
	if len(secret.OwnerReferences) == 0 {
		t.Error("Expected Secret to have OwnerReference to Pod")
	} else {
		ownerRef := secret.OwnerReferences[0]
		if ownerRef.Kind != "Pod" {
			t.Errorf("Expected OwnerReference kind 'Pod', got '%s'", ownerRef.Kind)
		}
		if ownerRef.Name != "e2e-test-pod" {
			t.Errorf("Expected OwnerReference name 'e2e-test-pod', got '%s'", ownerRef.Name)
		}
	}

	// Verify Secret contains decrypted data
	if secret.Data == nil {
		t.Error("Expected Secret to have data")
	} else {
		if _, exists := secret.Data["TEST_KEY"]; !exists {
			t.Error("Expected Secret to contain TEST_KEY")
		} else {
			decryptedValue := string(secret.Data["TEST_KEY"])
			if decryptedValue != "test-secret-value" {
				t.Errorf("Expected decrypted value 'test-secret-value', got '%s'", decryptedValue)
			}
		}
	}
}

func TestAllowedSubjects_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := context.Background()
	namespace := "default"

	// Create namespace with zen-lock label for webhook
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				"zen-lock": "enabled",
			},
		},
	}
	_ = k8sClient.Create(ctx, ns) // Ignore error if exists

	privateKey, publicKey := generateTestKeys(t)
	encryptedValue := encryptTestData(t, "test-value", publicKey)

	// Set private key for webhook and controller
	os.Setenv("ZEN_LOCK_PRIVATE_KEY", privateKey)
	defer os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")

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
				"key1": encryptedValue,
			},
			Algorithm: "age",
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

	// Wait for controller to reconcile
	time.Sleep(2 * time.Second)

	// Create Pod with allowed ServiceAccount
	allowedPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "e2e-allowed-pod",
			Namespace: namespace,
			Annotations: map[string]string{
				"zen-lock/inject": "e2e-allowed-subjects-test",
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "e2e-test-sa",
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

	// Create Pod - should succeed
	if err := k8sClient.Create(ctx, allowedPod); err != nil {
		t.Fatalf("Failed to create allowed Pod: %v", err)
	}
	defer func() {
		k8sClient.Delete(ctx, allowedPod)
	}()

	// Wait for webhook to process
	time.Sleep(2 * time.Second)

	// Verify Pod was mutated (injection succeeded)
	retrievedPod := &corev1.Pod{}
	nn := types.NamespacedName{Name: "e2e-allowed-pod", Namespace: namespace}
	if err := k8sClient.Get(ctx, nn, retrievedPod); err != nil {
		t.Fatalf("Failed to get Pod: %v", err)
	}

	// Check that volume was added (injection succeeded)
	foundVolume := false
	for _, vol := range retrievedPod.Spec.Volumes {
		if vol.Name == "zen-lock-e2e-allowed-subjects-test" {
			foundVolume = true
			break
		}
	}
	if !foundVolume {
		t.Error("Expected Pod to have zen-lock volume injected (allowed ServiceAccount)")
	}

	// Create Pod with disallowed ServiceAccount
	disallowedPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "e2e-disallowed-pod",
			Namespace: namespace,
			Annotations: map[string]string{
				"zen-lock/inject": "e2e-allowed-subjects-test",
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "default", // Not in AllowedSubjects
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

	// Create Pod - should fail (webhook should deny)
	if err := k8sClient.Create(ctx, disallowedPod); err == nil {
		// If creation succeeded, verify Pod was NOT mutated
		defer func() {
			k8sClient.Delete(ctx, disallowedPod)
		}()

		time.Sleep(2 * time.Second)

		retrievedDisallowedPod := &corev1.Pod{}
		disallowedNN := types.NamespacedName{Name: "e2e-disallowed-pod", Namespace: namespace}
		if err := k8sClient.Get(ctx, disallowedNN, retrievedDisallowedPod); err == nil {
			// Check that volume was NOT added (injection failed)
			for _, vol := range retrievedDisallowedPod.Spec.Volumes {
				if vol.Name == "zen-lock-e2e-allowed-subjects-test" {
					t.Error("Expected Pod to NOT have zen-lock volume injected (disallowed ServiceAccount)")
					break
				}
			}
		}
	} else {
		// Expected: webhook denied the request
		t.Logf("Pod creation denied as expected: %v", err)
	}
}

func TestZenLock_InvalidCiphertext(t *testing.T) {
	ctx := context.Background()
	namespace := "default"

	// Create namespace with zen-lock label for webhook
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				"zen-lock": "enabled",
			},
		},
	}
	_ = k8sClient.Create(ctx, ns) // Ignore error if exists

	privateKey, _ := generateTestKeys(t)

	// Set private key for controller
	os.Setenv("ZEN_LOCK_PRIVATE_KEY", privateKey)
	defer os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")

	// Create ZenLock with invalid ciphertext
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "e2e-invalid-ciphertext",
			Namespace: namespace,
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": "invalid-ciphertext",
			},
			Algorithm: "age",
		},
	}

	if err := k8sClient.Create(ctx, zenlock); err != nil {
		t.Fatalf("Failed to create ZenLock: %v", err)
	}
	defer func() {
		k8sClient.Delete(ctx, zenlock)
	}()

	// Wait for controller to reconcile
	time.Sleep(3 * time.Second)

	// Verify status shows error
	retrieved := &securityv1alpha1.ZenLock{}
	nn := types.NamespacedName{Name: "e2e-invalid-ciphertext", Namespace: namespace}
	if err := k8sClient.Get(ctx, nn, retrieved); err != nil {
		t.Fatalf("Failed to get ZenLock: %v", err)
	}

	// Check that status reflects error
	errorCondition := findCondition(retrieved.Status.Conditions, "Decryptable")
	if errorCondition != nil && errorCondition.Status == "True" {
		t.Error("Expected Decryptable condition to be False for invalid ciphertext")
	}
}

// Helper function to find a condition by type
func findCondition(conditions []securityv1alpha1.ZenLockCondition, conditionType string) *securityv1alpha1.ZenLockCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}
