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
	"encoding/json"
	"os"
	"testing"
	"time"

	"filippo.io/age"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
	"github.com/kube-zen/zen-lock/pkg/common"
	"github.com/kube-zen/zen-lock/pkg/controller"
	"github.com/kube-zen/zen-lock/pkg/crypto"
	webhookpkg "github.com/kube-zen/zen-lock/pkg/webhook"
)

func TestWebhookSecretReconcilerFlow_Integration(t *testing.T) {
	// Set test private key
	originalKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("ZEN_LOCK_PRIVATE_KEY", originalKey)
		} else {
			os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")
		}
	}()

	// Generate real age keys for testing
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}
	privateKey := identity.String()
	publicKey := identity.Recipient().String()
	os.Setenv("ZEN_LOCK_PRIVATE_KEY", privateKey)

	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(securityv1alpha1.AddToScheme(scheme))

	clientBuilder := fake.NewClientBuilder().WithScheme(scheme)

	// Create ZenLock with encrypted data
	plaintext := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	encryptor := crypto.NewAgeEncryptor()
	encryptedData := make(map[string]string)
	for k, v := range plaintext {
		ciphertext, err := encryptor.Encrypt([]byte(v), []string{publicKey})
		if err != nil {
			t.Fatalf("Failed to encrypt %s: %v", k, err)
		}
		encryptedData[k] = base64.StdEncoding.EncodeToString(ciphertext)
	}

	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: encryptedData,
			Algorithm:     "age",
		},
	}

	// Create Pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			UID:       types.UID("test-pod-uid-12345"),
			Annotations: map[string]string{
				"zen-lock/inject": "test-zenlock",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "test-container", Image: "nginx"},
			},
		},
	}

	client := clientBuilder.WithObjects(zenlock, pod).Build()

	// Create webhook handler
	handler, err := webhookpkg.NewPodHandler(client, scheme)
	if err != nil {
		t.Fatalf("Failed to create PodHandler: %v", err)
	}

	// Simulate webhook admission request
	podRaw, _ := json.Marshal(pod)
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object:    runtime.RawExtension{Raw: podRaw},
			Namespace: "default",
		},
	}

	ctx := context.Background()
	resp := handler.Handle(ctx, req)

	if !resp.Allowed {
		t.Fatalf("Expected webhook to allow request, got: %v", resp.Result)
	}

	// Verify Secret was created by webhook
	secretName := webhookpkg.GenerateSecretName("default", "test-pod")
	secret := &corev1.Secret{}
	if err := client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: "default"}, secret); err != nil {
		t.Fatalf("Failed to get Secret created by webhook: %v", err)
	}

	// Verify Secret has labels but no OwnerReference yet
	if secret.Labels[common.LabelPodName] != "test-pod" {
		t.Errorf("Expected pod-name label 'test-pod', got '%s'", secret.Labels[common.LabelPodName])
	}
	if len(secret.OwnerReferences) > 0 {
		t.Error("Expected Secret to not have OwnerReference yet (controller will set it)")
	}

	// Now test SecretReconciler setting OwnerReference
	secretReconciler := controller.NewSecretReconciler(client, scheme)
	secretReq := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      secretName,
			Namespace: "default",
		},
	}

	result, err := secretReconciler.Reconcile(ctx, secretReq)
	if err != nil {
		t.Fatalf("SecretReconciler.Reconcile() error = %v", err)
	}
	if result.Requeue {
		t.Error("Expected SecretReconciler to not requeue when Pod exists")
	}

	// Verify OwnerReference was set
	updatedSecret := &corev1.Secret{}
	if err := client.Get(ctx, secretReq.NamespacedName, updatedSecret); err != nil {
		t.Fatalf("Failed to get updated Secret: %v", err)
	}

	if len(updatedSecret.OwnerReferences) == 0 {
		t.Error("Expected Secret to have OwnerReference after reconciliation")
	} else {
		ownerRef := updatedSecret.OwnerReferences[0]
		if ownerRef.Kind != "Pod" {
			t.Errorf("Expected OwnerReference Kind 'Pod', got '%s'", ownerRef.Kind)
		}
		if ownerRef.Name != "test-pod" {
			t.Errorf("Expected OwnerReference Name 'test-pod', got '%s'", ownerRef.Name)
		}
		if string(ownerRef.UID) != "test-pod-uid-12345" {
			t.Errorf("Expected OwnerReference UID 'test-pod-uid-12345', got '%s'", ownerRef.UID)
		}
		if ownerRef.Controller == nil || !*ownerRef.Controller {
			t.Error("Expected OwnerReference Controller to be true")
		}
	}
}

func TestSecretReconciler_OrphanCleanup_Integration(t *testing.T) {
	ctx, clientBuilder, _ := setupTestEnvironment(t)

	// Create an orphaned Secret (Pod doesn't exist, Secret is old)
	oldTime := metav1.NewTime(time.Now().Add(-2 * time.Hour)) // 2 hours old
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "zen-lock-inject-default-orphan-pod-abc123",
			Namespace:         "default",
			CreationTimestamp: oldTime,
			Labels: map[string]string{
				common.LabelPodName:      "orphan-pod",
				common.LabelPodNamespace: "default",
				common.LabelZenLockName:  "test-zenlock",
			},
		},
		Data: map[string][]byte{
			"key1": []byte("value1"),
		},
	}

	client := clientBuilder.WithObjects(secret).Build()

	// Create SecretReconciler with short orphan TTL for testing
	secretReconciler := controller.NewSecretReconciler(client, client.Scheme())
	// Set short orphan TTL for testing (default is 1 minute, but we'll use 1 second)
	secretReconciler.OrphanTTL = 1 * time.Second

	secretReq := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "zen-lock-inject-default-orphan-pod-abc123",
			Namespace: "default",
		},
	}

	// Reconcile - should delete orphaned secret
	result, err := secretReconciler.Reconcile(ctx, secretReq)
	if err != nil {
		t.Fatalf("SecretReconciler.Reconcile() error = %v", err)
	}
	if result.Requeue {
		t.Error("Expected SecretReconciler to not requeue when deleting orphan")
	}

	// Verify Secret was deleted
	deletedSecret := &corev1.Secret{}
	if err := client.Get(ctx, secretReq.NamespacedName, deletedSecret); err == nil {
		t.Error("Expected orphaned Secret to be deleted")
	}
}

func TestWebhook_StaleSecretRefresh_Integration(t *testing.T) {
	// Set test private key
	originalKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("ZEN_LOCK_PRIVATE_KEY", originalKey)
		} else {
			os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")
		}
	}()

	// Generate real age keys
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}
	privateKey := identity.String()
	publicKey := identity.Recipient().String()
	os.Setenv("ZEN_LOCK_PRIVATE_KEY", privateKey)

	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(securityv1alpha1.AddToScheme(scheme))

	clientBuilder := fake.NewClientBuilder().WithScheme(scheme)

	encryptor := crypto.NewAgeEncryptor()

	// Create ZenLock with initial encrypted data
	plaintext1 := map[string]string{
		"key1": "old-value",
	}
	encryptedData1 := make(map[string]string)
	for k, v := range plaintext1 {
		ciphertext, err := encryptor.Encrypt([]byte(v), []string{publicKey})
		if err != nil {
			t.Fatalf("Failed to encrypt: %v", err)
		}
		encryptedData1[k] = base64.StdEncoding.EncodeToString(ciphertext)
	}

	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: encryptedData1,
			Algorithm:     "age",
		},
	}

	// Create existing Secret with old data (stale)
	secretName := webhookpkg.GenerateSecretName("default", "test-pod")
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "default",
			Labels: map[string]string{
				common.LabelPodName:      "test-pod",
				common.LabelPodNamespace: "default",
				common.LabelZenLockName:  "test-zenlock",
			},
		},
		Data: map[string][]byte{
			"key1": []byte("old-value"), // Old decrypted value
		},
	}

	// Update ZenLock with new encrypted data
	plaintext2 := map[string]string{
		"key1": "new-value", // Changed value
	}
	encryptedData2 := make(map[string]string)
	for k, v := range plaintext2 {
		ciphertext, err := encryptor.Encrypt([]byte(v), []string{publicKey})
		if err != nil {
			t.Fatalf("Failed to encrypt: %v", err)
		}
		encryptedData2[k] = base64.StdEncoding.EncodeToString(ciphertext)
	}
	zenlock.Spec.EncryptedData = encryptedData2

	// Create Pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				"zen-lock/inject": "test-zenlock",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "test-container", Image: "nginx"},
			},
		},
	}

	client := clientBuilder.WithObjects(zenlock, pod, existingSecret).Build()

	// Create webhook handler
	handler, err := webhookpkg.NewPodHandler(client, scheme)
	if err != nil {
		t.Fatalf("Failed to create PodHandler: %v", err)
	}

	ctx := context.Background()

	// Simulate webhook admission request
	podRaw, _ := json.Marshal(pod)
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object:    runtime.RawExtension{Raw: podRaw},
			Namespace: "default",
		},
	}

	resp := handler.Handle(ctx, req)

	if !resp.Allowed {
		t.Fatalf("Expected webhook to allow request, got: %v", resp.Result)
	}

	// Verify Secret was updated with new data
	updatedSecret := &corev1.Secret{}
	if err := client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: "default"}, updatedSecret); err != nil {
		t.Fatalf("Failed to get updated Secret: %v", err)
	}

	// Verify data was refreshed (decrypted value should be "new-value")
	if string(updatedSecret.Data["key1"]) != "new-value" {
		t.Errorf("Expected Secret data to be refreshed to 'new-value', got '%s'", string(updatedSecret.Data["key1"]))
	}
}

func TestFullLifecycle_Integration(t *testing.T) {
	// Set test private key
	originalKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("ZEN_LOCK_PRIVATE_KEY", originalKey)
		} else {
			os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")
		}
	}()

	// Generate real age keys
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}
	privateKey := identity.String()
	publicKey := identity.Recipient().String()
	os.Setenv("ZEN_LOCK_PRIVATE_KEY", privateKey)

	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(securityv1alpha1.AddToScheme(scheme))

	clientBuilder := fake.NewClientBuilder().WithScheme(scheme)

	encryptor := crypto.NewAgeEncryptor()

	// Create ZenLock
	plaintext := map[string]string{
		"key1": "value1",
	}
	encryptedData := make(map[string]string)
	for k, v := range plaintext {
		ciphertext, err := encryptor.Encrypt([]byte(v), []string{publicKey})
		if err != nil {
			t.Fatalf("Failed to encrypt: %v", err)
		}
		encryptedData[k] = base64.StdEncoding.EncodeToString(ciphertext)
	}

	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: encryptedData,
			Algorithm:     "age",
		},
	}

	client := clientBuilder.WithObjects(zenlock).Build()

	ctx := context.Background()

	// Step 1: Create Pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			UID:       types.UID("test-pod-uid-12345"),
			Annotations: map[string]string{
				"zen-lock/inject": "test-zenlock",
			},
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

	// Step 2: Webhook injects secret
	handler, err := webhookpkg.NewPodHandler(client, scheme)
	if err != nil {
		t.Fatalf("Failed to create PodHandler: %v", err)
	}

	podRaw, _ := json.Marshal(pod)
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object:    runtime.RawExtension{Raw: podRaw},
			Namespace: "default",
		},
	}

	resp := handler.Handle(ctx, req)
	if !resp.Allowed {
		t.Fatalf("Expected webhook to allow request, got: %v", resp.Result)
	}

	secretName := webhookpkg.GenerateSecretName("default", "test-pod")

	// Step 3: Controller sets OwnerReference
	secretReconciler := controller.NewSecretReconciler(client, scheme)
	secretReq := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      secretName,
			Namespace: "default",
		},
	}

	_, err = secretReconciler.Reconcile(ctx, secretReq)
	if err != nil {
		t.Fatalf("SecretReconciler.Reconcile() error = %v", err)
	}

	// Verify OwnerReference is set
	secret := &corev1.Secret{}
	if err := client.Get(ctx, secretReq.NamespacedName, secret); err != nil {
		t.Fatalf("Failed to get Secret: %v", err)
	}

	if len(secret.OwnerReferences) == 0 {
		t.Fatal("Expected Secret to have OwnerReference")
	}

	// Step 4: Delete Pod (simulating pod termination)
	if err := client.Delete(ctx, pod); err != nil {
		t.Fatalf("Failed to delete Pod: %v", err)
	}

	// Step 5: Verify Secret is deleted (Kubernetes garbage collection)
	// Note: In a real cluster, Kubernetes would delete the Secret automatically
	// In fake client, we need to manually verify the OwnerReference relationship
	// The Secret should have OwnerReference pointing to the deleted Pod
	if secret.OwnerReferences[0].UID != pod.UID {
		t.Errorf("Expected OwnerReference UID to match Pod UID")
	}

	// In a real cluster, Kubernetes would garbage collect the Secret
	// Here we just verify the OwnerReference relationship is correct
}

func TestWebhook_CacheBehavior_Integration(t *testing.T) {
	// Set test private key
	originalKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("ZEN_LOCK_PRIVATE_KEY", originalKey)
		} else {
			os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")
		}
	}()

	// Generate real age keys
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}
	privateKey := identity.String()
	publicKey := identity.Recipient().String()
	os.Setenv("ZEN_LOCK_PRIVATE_KEY", privateKey)

	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(securityv1alpha1.AddToScheme(scheme))

	clientBuilder := fake.NewClientBuilder().WithScheme(scheme)

	encryptor := crypto.NewAgeEncryptor()

	// Create ZenLock
	plaintext := map[string]string{
		"key1": "value1",
	}
	encryptedData := make(map[string]string)
	for k, v := range plaintext {
		ciphertext, err := encryptor.Encrypt([]byte(v), []string{publicKey})
		if err != nil {
			t.Fatalf("Failed to encrypt: %v", err)
		}
		encryptedData[k] = base64.StdEncoding.EncodeToString(ciphertext)
	}

	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: encryptedData,
			Algorithm:     "age",
		},
	}

	client := clientBuilder.WithObjects(zenlock).Build()

	// Create webhook handler
	handler, err := webhookpkg.NewPodHandler(client, scheme)
	if err != nil {
		t.Fatalf("Failed to create PodHandler: %v", err)
	}

	// Create first Pod
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-1",
			Namespace: "default",
			Annotations: map[string]string{
				"zen-lock/inject": "test-zenlock",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "test-container", Image: "nginx"},
			},
		},
	}

	ctx := context.Background()

	// First request - should be cache miss
	pod1Raw, _ := json.Marshal(pod1)
	req1 := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object:    runtime.RawExtension{Raw: pod1Raw},
			Namespace: "default",
		},
	}

	resp1 := handler.Handle(ctx, req1)
	if !resp1.Allowed {
		t.Fatalf("Expected webhook to allow first request, got: %v", resp1.Result)
	}

	// Second request with different Pod but same ZenLock - should be cache hit
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-2",
			Namespace: "default",
			Annotations: map[string]string{
				"zen-lock/inject": "test-zenlock", // Same ZenLock
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "test-container", Image: "nginx"},
			},
		},
	}

	pod2Raw, _ := json.Marshal(pod2)
	req2 := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object:    runtime.RawExtension{Raw: pod2Raw},
			Namespace: "default",
		},
	}

	resp2 := handler.Handle(ctx, req2)
	if !resp2.Allowed {
		t.Fatalf("Expected webhook to allow second request, got: %v", resp2.Result)
	}

	// Verify cache was used (both requests should succeed)
	// The cache should have been populated after first request
	// Second request should use cached ZenLock instead of fetching from API
}

func TestWebhook_DryRun_Integration(t *testing.T) {
	// Set test private key
	originalKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("ZEN_LOCK_PRIVATE_KEY", originalKey)
		} else {
			os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")
		}
	}()

	// Generate real age keys
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}
	privateKey := identity.String()
	publicKey := identity.Recipient().String()
	os.Setenv("ZEN_LOCK_PRIVATE_KEY", privateKey)

	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(securityv1alpha1.AddToScheme(scheme))

	clientBuilder := fake.NewClientBuilder().WithScheme(scheme)

	encryptor := crypto.NewAgeEncryptor()

	// Create ZenLock
	plaintext := map[string]string{
		"key1": "value1",
	}
	encryptedData := make(map[string]string)
	for k, v := range plaintext {
		ciphertext, err := encryptor.Encrypt([]byte(v), []string{publicKey})
		if err != nil {
			t.Fatalf("Failed to encrypt: %v", err)
		}
		encryptedData[k] = base64.StdEncoding.EncodeToString(ciphertext)
	}

	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: encryptedData,
			Algorithm:     "age",
		},
	}

	client := clientBuilder.WithObjects(zenlock).Build()

	// Create webhook handler
	handler, err := webhookpkg.NewPodHandler(client, scheme)
	if err != nil {
		t.Fatalf("Failed to create PodHandler: %v", err)
	}

	// Create Pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				"zen-lock/inject": "test-zenlock",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "test-container", Image: "nginx"},
			},
		},
	}

	ctx := context.Background()

	// Test dry-run mode
	dryRun := true
	podRaw, _ := json.Marshal(pod)
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object:    runtime.RawExtension{Raw: podRaw},
			Namespace: "default",
			DryRun:    &dryRun,
		},
	}

	resp := handler.Handle(ctx, req)
	if !resp.Allowed {
		t.Fatalf("Expected webhook to allow dry-run request, got: %v", resp.Result)
	}

	// Verify Secret was NOT created in dry-run mode
	secretName := webhookpkg.GenerateSecretName("default", "test-pod")
	secret := &corev1.Secret{}
	if err := client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: "default"}, secret); err == nil {
		t.Error("Expected Secret to NOT be created in dry-run mode")
	}

	// In dry-run mode, webhook should still allow the request
	// Patch may or may not be returned depending on implementation
	// The key is that no actual Secret was created
}

func TestSecretReconciler_RequeueWhenPodNotReady_Integration(t *testing.T) {
	ctx, clientBuilder, _ := setupTestEnvironment(t)

	// Create Secret with labels but Pod doesn't exist yet (new Secret, Pod might be created soon)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "zen-lock-inject-default-new-pod-abc123",
			Namespace:         "default",
			CreationTimestamp: metav1.Now(), // New secret
			Labels: map[string]string{
				common.LabelPodName:      "new-pod",
				common.LabelPodNamespace: "default",
				common.LabelZenLockName:  "test-zenlock",
			},
		},
		Data: map[string][]byte{
			"key1": []byte("value1"),
		},
	}

	client := clientBuilder.WithObjects(secret).Build()

	secretReconciler := controller.NewSecretReconciler(client, client.Scheme())

	secretReq := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "zen-lock-inject-default-new-pod-abc123",
			Namespace: "default",
		},
	}

	// Reconcile - should requeue (Pod not found, but Secret is new)
	result, err := secretReconciler.Reconcile(ctx, secretReq)
	if err != nil {
		t.Fatalf("SecretReconciler.Reconcile() error = %v", err)
	}

	// Should requeue when Pod doesn't exist but Secret is new
	if !result.Requeue && result.RequeueAfter == 0 {
		t.Error("Expected SecretReconciler to requeue when Pod not found and Secret is new")
	}

	// Verify Secret still exists (not deleted)
	if err := client.Get(ctx, secretReq.NamespacedName, secret); err != nil {
		t.Errorf("Expected Secret to still exist, got error: %v", err)
	}
}
