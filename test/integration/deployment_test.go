//go:build integration
// +build integration

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
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
	webhookpkg "github.com/kube-zen/zen-lock/pkg/webhook"
)

var (
	k8sClient     client.Client
	clientset     *kubernetes.Clientset
	scheme        = runtime.NewScheme()
	testNamespace = "zen-lock-test"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(securityv1alpha1.AddToScheme(scheme))
}

// setupKubernetesClient sets up a Kubernetes client from kubeconfig
func setupKubernetesClient(t *testing.T) {
	t.Helper()

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		// Try default kubeconfig locations
		home, err := os.UserHomeDir()
		if err == nil {
			kubeconfig = fmt.Sprintf("%s/.kube/zen-lock-integration-config", home)
		}
	}

	if kubeconfig == "" || !fileExists(kubeconfig) {
		t.Skip("KUBECONFIG not set or file not found. Run: test/integration/setup_kind.sh create")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatalf("Failed to build config: %v", err)
	}

	// Create client (REST mapper is created automatically)
	k8sClient, err = client.New(config, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create clientset
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("Failed to create clientset: %v", err)
	}
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

// generateTestKeys generates a test key pair
func generateTestKeys(t *testing.T) (privateKey, publicKey string) {
	t.Helper()
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate test identity: %v", err)
	}
	return identity.String(), identity.Recipient().String()
}

// encryptTestData encrypts test data using the provided public key
func encryptTestData(t *testing.T, plaintext string, publicKey string) string {
	t.Helper()
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

// ensureNamespace creates a test namespace if it doesn't exist
func ensureNamespace(ctx context.Context, t *testing.T) {
	t.Helper()
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace,
		},
	}

	if err := k8sClient.Get(ctx, types.NamespacedName{Name: testNamespace}, ns); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// Namespace doesn't exist, create it
			if err := k8sClient.Create(ctx, ns); err != nil {
				t.Fatalf("Failed to create namespace: %v", err)
			}
			t.Logf("Created test namespace: %s", testNamespace)
		} else {
			t.Fatalf("Failed to check namespace: %v", err)
		}
	}
}

// waitForDeployment waits for a deployment to be ready
func waitForDeployment(ctx context.Context, t *testing.T, name, namespace string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			if deployment.Status.ReadyReplicas > 0 {
				t.Logf("Deployment %s/%s is ready", namespace, name)
				return
			}
		}
		time.Sleep(1 * time.Second)
	}
	t.Fatalf("Deployment %s/%s not ready after %v", namespace, name, timeout)
}

// TestZenLockDeployment validates that zen-lock is deployed and running
func TestZenLockDeployment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping deployment test in short mode")
	}

	setupKubernetesClient(t)
	ctx := context.Background()

	// Check webhook deployment
	t.Run("WebhookDeployment", func(t *testing.T) {
		waitForDeployment(ctx, t, "zen-lock-webhook", "zen-lock-system", 2*time.Minute)

		// Check pods
		pods, err := clientset.CoreV1().Pods("zen-lock-system").List(ctx, metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/name=zen-lock,app.kubernetes.io/component=webhook",
		})
		if err != nil {
			t.Fatalf("Failed to list webhook pods: %v", err)
		}
		if len(pods.Items) == 0 {
			t.Fatal("No webhook pods found")
		}

		// Check pod is running
		for _, pod := range pods.Items {
			if pod.Status.Phase != corev1.PodRunning {
				t.Errorf("Webhook pod %s is not running: %s", pod.Name, pod.Status.Phase)
			}
		}
	})

	// Check controller deployment
	t.Run("ControllerDeployment", func(t *testing.T) {
		waitForDeployment(ctx, t, "zen-lock-controller", "zen-lock-system", 2*time.Minute)

		// Check pods
		pods, err := clientset.CoreV1().Pods("zen-lock-system").List(ctx, metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/name=zen-lock,app.kubernetes.io/component=controller",
		})
		if err != nil {
			t.Fatalf("Failed to list controller pods: %v", err)
		}
		if len(pods.Items) == 0 {
			t.Fatal("No controller pods found")
		}

		// Check pod is running
		for _, pod := range pods.Items {
			if pod.Status.Phase != corev1.PodRunning {
				t.Errorf("Controller pod %s is not running: %s", pod.Name, pod.Status.Phase)
			}
		}
	})

	// Check webhook configuration
	t.Run("WebhookConfiguration", func(t *testing.T) {
		webhookConfigs, err := clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().List(ctx, metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/name=zen-lock",
		})
		if err != nil {
			t.Fatalf("Failed to list webhook configurations: %v", err)
		}
		if len(webhookConfigs.Items) == 0 {
			t.Fatal("No webhook configurations found")
		}
	})

	// Check CRD exists
	t.Run("CRDExists", func(t *testing.T) {
		zenlock := &securityv1alpha1.ZenLock{}
		zenlock.Name = "test-crd-check"
		zenlock.Namespace = testNamespace

		// Try to create a ZenLock to verify CRD exists
		// We'll delete it immediately
		ensureNamespace(ctx, t)
		if err := k8sClient.Create(ctx, zenlock); err == nil {
			// CRD exists and we can create resources
			_ = k8sClient.Delete(ctx, zenlock)
		} else {
			t.Fatalf("CRD may not be installed or accessible: %v", err)
		}
	})
}

// TestZenLockFullLifecycle tests the complete lifecycle: create ZenLock, inject into Pod, verify secret
func TestZenLockFullLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping full lifecycle test in short mode")
	}

	setupKubernetesClient(t)
	ctx := context.Background()
	ensureNamespace(ctx, t)

	_, publicKey := generateTestKeys(t)
	encryptedValue := encryptTestData(t, "test-secret-value", publicKey)

	// Create ZenLock
	zenlockName := "integration-test-zenlock"
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zenlockName,
			Namespace: testNamespace,
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
		_ = k8sClient.Delete(ctx, zenlock)
	}()

	// Wait for controller to reconcile
	time.Sleep(3 * time.Second)

	// Verify ZenLock status
	retrieved := &securityv1alpha1.ZenLock{}
	nn := types.NamespacedName{Name: zenlockName, Namespace: testNamespace}
	if err := k8sClient.Get(ctx, nn, retrieved); err != nil {
		t.Fatalf("Failed to get ZenLock: %v", err)
	}

	// Create Pod with injection annotation
	podName := "integration-test-pod"
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: testNamespace,
			Annotations: map[string]string{
				"zen-lock/inject": zenlockName,
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

	if err := k8sClient.Create(ctx, pod); err != nil {
		t.Fatalf("Failed to create Pod: %v", err)
	}
	defer func() {
		_ = k8sClient.Delete(ctx, pod)
	}()

	// Wait for webhook to process
	time.Sleep(3 * time.Second)

	// Verify Pod was mutated
	retrievedPod := &corev1.Pod{}
	podNN := types.NamespacedName{Name: podName, Namespace: testNamespace}
	if err := k8sClient.Get(ctx, podNN, retrievedPod); err != nil {
		t.Fatalf("Failed to get Pod: %v", err)
	}

	// Check volume was added
	foundVolume := false
	expectedSecretName := webhookpkg.GenerateSecretName(testNamespace, podName)
	for _, vol := range retrievedPod.Spec.Volumes {
		if vol.Name == "zen-secrets" {
			foundVolume = true
			if vol.Secret == nil || vol.Secret.SecretName != expectedSecretName {
				t.Errorf("Expected volume to reference secret '%s', got '%v'", expectedSecretName, vol.Secret)
			}
			break
		}
	}
	if !foundVolume {
		t.Error("Expected Pod to have zen-lock volume injected")
	}

	// Verify ephemeral Secret was created
	secret := &corev1.Secret{}
	secretNN := types.NamespacedName{Name: expectedSecretName, Namespace: testNamespace}
	if err := k8sClient.Get(ctx, secretNN, secret); err != nil {
		t.Fatalf("Failed to get ephemeral Secret: %v", err)
	}

	// Verify Secret contains decrypted data
	if secret.Data == nil {
		t.Error("Expected Secret to have data")
	} else {
		if value, exists := secret.Data["TEST_KEY"]; !exists {
			t.Error("Expected Secret to contain TEST_KEY")
		} else {
			decryptedValue := string(value)
			if decryptedValue != "test-secret-value" {
				t.Errorf("Expected decrypted value 'test-secret-value', got '%s'", decryptedValue)
			}
		}
	}
}

// TestZenLockAllowedSubjects tests AllowedSubjects validation
func TestZenLockAllowedSubjects(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping AllowedSubjects test in short mode")
	}

	setupKubernetesClient(t)
	ctx := context.Background()
	ensureNamespace(ctx, t)

	_, publicKey := generateTestKeys(t)
	encryptedValue := encryptTestData(t, "test-value", publicKey)

	// Create ServiceAccount
	saName := "integration-test-sa"
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: testNamespace,
		},
	}

	if err := k8sClient.Create(ctx, sa); err != nil {
		t.Fatalf("Failed to create ServiceAccount: %v", err)
	}
	defer func() {
		_ = k8sClient.Delete(ctx, sa)
	}()

	// Create ZenLock with AllowedSubjects
	zenlockName := "integration-allowed-subjects-test"
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zenlockName,
			Namespace: testNamespace,
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": encryptedValue,
			},
			Algorithm: "age",
			AllowedSubjects: []securityv1alpha1.SubjectReference{
				{
					Kind:      "ServiceAccount",
					Name:      saName,
					Namespace: testNamespace,
				},
			},
		},
	}

	if err := k8sClient.Create(ctx, zenlock); err != nil {
		t.Fatalf("Failed to create ZenLock: %v", err)
	}
	defer func() {
		_ = k8sClient.Delete(ctx, zenlock)
	}()

	// Wait for controller to reconcile
	time.Sleep(3 * time.Second)

	// Create Pod with allowed ServiceAccount - should succeed
	allowedPodName := "integration-allowed-pod"
	allowedPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      allowedPodName,
			Namespace: testNamespace,
			Annotations: map[string]string{
				"zen-lock/inject": zenlockName,
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: saName,
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

	if err := k8sClient.Create(ctx, allowedPod); err != nil {
		t.Fatalf("Failed to create allowed Pod: %v", err)
	}
	defer func() {
		_ = k8sClient.Delete(ctx, allowedPod)
	}()

	// Wait for webhook to process
	time.Sleep(3 * time.Second)

	// Verify Pod was mutated (injection succeeded)
	retrievedPod := &corev1.Pod{}
	podNN := types.NamespacedName{Name: allowedPodName, Namespace: testNamespace}
	if err := k8sClient.Get(ctx, podNN, retrievedPod); err != nil {
		t.Fatalf("Failed to get Pod: %v", err)
	}

	// Check that volume was added (injection succeeded)
	foundVolume := false
	for _, vol := range retrievedPod.Spec.Volumes {
		if vol.Name == "zen-secrets" {
			foundVolume = true
			break
		}
	}
	if !foundVolume {
		t.Error("Expected Pod to have zen-lock volume injected (allowed ServiceAccount)")
	}

	// Create Pod with disallowed ServiceAccount - should be denied
	disallowedPodName := "integration-disallowed-pod"
	disallowedPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      disallowedPodName,
			Namespace: testNamespace,
			Annotations: map[string]string{
				"zen-lock/inject": zenlockName,
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

	// Try to create Pod - should fail
	if err := k8sClient.Create(ctx, disallowedPod); err == nil {
		// If creation succeeded, verify Pod was NOT mutated
		defer func() {
			_ = k8sClient.Delete(ctx, disallowedPod)
		}()

		time.Sleep(3 * time.Second)

		retrievedDisallowedPod := &corev1.Pod{}
		disallowedNN := types.NamespacedName{Name: disallowedPodName, Namespace: testNamespace}
		if err := k8sClient.Get(ctx, disallowedNN, retrievedDisallowedPod); err == nil {
			// Check that volume was NOT added (injection failed)
			for _, vol := range retrievedDisallowedPod.Spec.Volumes {
				if vol.Name == "zen-secrets" {
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

// TestZenLockControllerReconciliation tests controller reconciliation
func TestZenLockControllerReconciliation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping controller reconciliation test in short mode")
	}

	setupKubernetesClient(t)
	ctx := context.Background()
	ensureNamespace(ctx, t)

	_, publicKey := generateTestKeys(t)
	encryptedValue := encryptTestData(t, "test-value", publicKey)

	zenlockName := "integration-reconcile-test"
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zenlockName,
			Namespace: testNamespace,
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
		_ = k8sClient.Delete(ctx, zenlock)
	}()

	// Wait for controller to reconcile - retry until status is updated
	nn := types.NamespacedName{Name: zenlockName, Namespace: testNamespace}
	var retrieved *securityv1alpha1.ZenLock
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
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

// findCondition finds a condition by type
func findCondition(conditions []securityv1alpha1.ZenLockCondition, conditionType string) *securityv1alpha1.ZenLockCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

// TestZenLockSecretCleanup tests that secrets are cleaned up when Pods are deleted
func TestZenLockSecretCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping secret cleanup test in short mode")
	}

	setupKubernetesClient(t)
	ctx := context.Background()
	ensureNamespace(ctx, t)

	_, publicKey := generateTestKeys(t)
	encryptedValue := encryptTestData(t, "test-secret-value", publicKey)

	// Create ZenLock
	zenlockName := "integration-cleanup-test"
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zenlockName,
			Namespace: testNamespace,
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
		_ = k8sClient.Delete(ctx, zenlock)
	}()

	// Wait for controller to reconcile
	time.Sleep(3 * time.Second)

	// Create Pod
	podName := "integration-cleanup-pod"
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: testNamespace,
			Annotations: map[string]string{
				"zen-lock/inject": zenlockName,
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

	if err := k8sClient.Create(ctx, pod); err != nil {
		t.Fatalf("Failed to create Pod: %v", err)
	}

	// Wait for webhook to process
	time.Sleep(3 * time.Second)

	// Verify Secret was created
	expectedSecretName := webhookpkg.GenerateSecretName(testNamespace, podName)
	secret := &corev1.Secret{}
	secretNN := types.NamespacedName{Name: expectedSecretName, Namespace: testNamespace}
	if err := k8sClient.Get(ctx, secretNN, secret); err != nil {
		t.Fatalf("Failed to get ephemeral Secret: %v", err)
	}

	// Delete Pod
	if err := k8sClient.Delete(ctx, pod); err != nil {
		t.Fatalf("Failed to delete Pod: %v", err)
	}

	// Wait for controller to clean up (OwnerReference should trigger deletion)
	time.Sleep(5 * time.Second)

	// Verify Secret was deleted
	deletedSecret := &corev1.Secret{}
	if err := k8sClient.Get(ctx, secretNN, deletedSecret); err == nil {
		t.Error("Expected Secret to be deleted after Pod deletion")
	}
}

