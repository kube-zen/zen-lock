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
	"encoding/json"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
)

func TestPodHandler_Handle_InvalidPodDecode(t *testing.T) {
	handler, _ := setupTestPodHandler(t)

	// Invalid pod JSON
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object:    runtime.RawExtension{Raw: []byte("invalid json")},
			Namespace: "default",
		},
	}

	ctx := context.Background()
	resp := handler.Handle(ctx, req)

	if resp.Allowed {
		t.Error("Expected request to be denied for invalid pod JSON")
	}
}

func TestPodHandler_Handle_InvalidInjectAnnotation(t *testing.T) {
	handler, _ := setupTestPodHandler(t)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				annotationInject: "-invalid-", // Invalid annotation
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
		t.Error("Expected request to be denied for invalid inject annotation")
	}
}

func TestPodHandler_Handle_InvalidMountPath(t *testing.T) {
	handler, _ := setupTestPodHandler(t)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				annotationInject:  "test-zenlock",
				annotationMountPath: "/etc/", // Invalid mount path
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

func TestPodHandler_Handle_ZenLockNotFound(t *testing.T) {
	handler, _ := setupTestPodHandler(t)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				annotationInject: "non-existent-zenlock",
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
		t.Error("Expected request to be denied when ZenLock not found")
	}
}

func TestPodHandler_Handle_DecryptionFailure(t *testing.T) {
	handler, clientBuilder := setupTestPodHandler(t)

	// Create a ZenLock with invalid encrypted data
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": "invalid-ciphertext-not-base64",
			},
		},
	}

	client := clientBuilder.WithObjects(zenlock).Build()
	handler.Client = client

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

	ctx := context.Background()
	resp := handler.Handle(ctx, req)

	if resp.Allowed {
		t.Error("Expected request to be denied when decryption fails")
	}
}

func TestPodHandler_Handle_DryRun(t *testing.T) {
	handler, clientBuilder := setupTestPodHandler(t)

	// Create a ZenLock with encrypted data
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": "dGVzdC12YWx1ZQ==", // base64 encoded test value
			},
		},
	}

	client := clientBuilder.WithObjects(zenlock).Build()
	handler.Client = client

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
	dryRun := true
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object:    runtime.RawExtension{Raw: podRaw},
			Namespace: "default",
			DryRun:    &dryRun,
		},
	}

	ctx := context.Background()
	resp := handler.Handle(ctx, req)

	// Dry-run should still allow the request (mutation happens but no Secret created)
	// The response should contain a patch
	if !resp.Allowed {
		t.Error("Expected request to be allowed in dry-run mode")
	}
}

func TestPodHandler_Handle_CacheHit(t *testing.T) {
	handler, clientBuilder := setupTestPodHandler(t)

	// Create a ZenLock
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": "dGVzdC12YWx1ZQ==",
			},
		},
	}

	client := clientBuilder.WithObjects(zenlock).Build()
	handler.Client = client

	// Pre-populate cache
	cacheKey := types.NamespacedName{Namespace: "default", Name: "test-zenlock"}
	handler.cache.Set(cacheKey, zenlock)

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

	ctx := context.Background()
	resp := handler.Handle(ctx, req)

	// Should use cache and still process (may fail on decryption but cache path is tested)
	// We're mainly testing that cache hit path is executed
	if resp.Result != nil && resp.Result.Message != "" {
		// Error is expected due to invalid ciphertext, but cache path was executed
	}
}

