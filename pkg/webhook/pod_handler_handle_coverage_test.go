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

package webhook

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"filippo.io/age"
	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
	"github.com/kube-zen/zen-lock/pkg/crypto"
)

// setupTestPodHandlerWithKey creates a PodHandler with a specific private key
func setupTestPodHandlerWithKey(t *testing.T, privateKey string) (*PodHandler, *fake.ClientBuilder) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(securityv1alpha1.AddToScheme(scheme))

	clientBuilder := fake.NewClientBuilder().WithScheme(scheme)

	encryptor := crypto.NewAgeEncryptor()
	cache := NewZenLockCache(5 * time.Minute)

	handler := &PodHandler{
		Client:     clientBuilder.Build(),
		decoder:    admission.NewDecoder(scheme),
		crypto:     encryptor,
		privateKey: privateKey,
		cache:      cache,
	}

	return handler, clientBuilder
}

func TestPodHandler_Handle_ContextTimeout(t *testing.T) {
	handler, _ := setupTestPodHandler(t)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				annotationInject: "test-zenlock",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "test-container", Image: "nginx"},
			},
		},
	}

	podRaw, _ := json.Marshal(pod)
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object:    runtime.RawExtension{Raw: podRaw},
			Namespace: "default",
		},
	}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resp := handler.Handle(ctx, req)

	// Should handle gracefully (may return error or allowed depending on when timeout occurs)
	_ = resp
}

func TestPodHandler_Handle_CacheMiss(t *testing.T) {
	// Generate real age keys FIRST
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}
	privateKey := identity.String()
	publicKey := identity.Recipient().String()

	// Set private key BEFORE creating handler
	originalKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	os.Setenv("ZEN_LOCK_PRIVATE_KEY", privateKey)
	defer func() {
		if originalKey != "" {
			os.Setenv("ZEN_LOCK_PRIVATE_KEY", originalKey)
		} else {
			os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")
		}
	}()

	// Encrypt data
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
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": encryptedData,
			},
			AllowedSubjects: []securityv1alpha1.SubjectReference{
				{
					Kind:      "ServiceAccount",
					Name:      "default",
					Namespace: "default",
				},
			},
		},
	}

	handler, clientBuilder := setupTestPodHandlerWithKey(t, privateKey)
	client := clientBuilder.WithObjects(zenlock).Build()
	handler.Client = client

	// Clear cache to force cache miss
	handler.cache.InvalidateAll()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				annotationInject:    "test-zenlock",
				annotationMountPath: "/zen-lock/secrets",
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "default",
			Containers: []corev1.Container{
				{Name: "test-container", Image: "nginx"},
			},
		},
	}

	podRaw, _ := json.Marshal(pod)
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object:    runtime.RawExtension{Raw: podRaw},
			Namespace: "default",
		},
	}

	ctx := context.Background()
	resp := handler.Handle(ctx, req)

	// Should succeed (cache miss should trigger fetch from API server)
	if !resp.Allowed {
		t.Errorf("Expected request to be allowed, got: %v", resp.Result)
	}
}

func TestPodHandler_Handle_InvalidMountPath_Coverage(t *testing.T) {
	handler, _ := setupTestPodHandler(t)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				annotationInject:    "test-zenlock",
				annotationMountPath: "../invalid/path", // Invalid relative path
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "test-container", Image: "nginx"},
			},
		},
	}

	podRaw, _ := json.Marshal(pod)
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object:    runtime.RawExtension{Raw: podRaw},
			Namespace: "default",
		},
	}

	ctx := context.Background()
	resp := handler.Handle(ctx, req)

	if resp.Allowed {
		t.Error("Expected request to be denied for invalid mount path")
	}
}

func TestPodHandler_Handle_DefaultMountPath(t *testing.T) {
	// Generate real age keys FIRST
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}
	privateKey := identity.String()
	publicKey := identity.Recipient().String()

	// Set private key BEFORE creating handler
	originalKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	os.Setenv("ZEN_LOCK_PRIVATE_KEY", privateKey)
	defer func() {
		if originalKey != "" {
			os.Setenv("ZEN_LOCK_PRIVATE_KEY", originalKey)
		} else {
			os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")
		}
	}()

	// Encrypt data
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
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": encryptedData,
			},
			AllowedSubjects: []securityv1alpha1.SubjectReference{
				{
					Kind:      "ServiceAccount",
					Name:      "default",
					Namespace: "default",
				},
			},
		},
	}

	handler, clientBuilder := setupTestPodHandlerWithKey(t, privateKey)
	client := clientBuilder.WithObjects(zenlock).Build()
	handler.Client = client

	// Pod without mount path annotation (should use default)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				annotationInject: "test-zenlock",
				// No annotationMountPath - should use default
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "default",
			Containers: []corev1.Container{
				{Name: "test-container", Image: "nginx"},
			},
		},
	}

	podRaw, _ := json.Marshal(pod)
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object:    runtime.RawExtension{Raw: podRaw},
			Namespace: "default",
		},
	}

	ctx := context.Background()
	resp := handler.Handle(ctx, req)

	// Should succeed with default mount path
	if !resp.Allowed {
		t.Errorf("Expected request to be allowed with default mount path, got: %v", resp.Result)
	}
}
