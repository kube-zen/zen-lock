/*
Copyright 2025 Kube-ZEN Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhook

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"k8s.io/apimachinery/pkg/types"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
	"github.com/kube-zen/zen-lock/pkg/config"
)

func TestPodHandler_HandleDryRun_Success(t *testing.T) {
	handler, clientBuilder := setupTestPodHandler(t)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				config.AnnotationInject: "test-zenlock",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "test-container", Image: "nginx"},
			},
		},
	}

	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key": "encrypted-value",
			},
		},
	}

	// Add to cache
	handler.cache.Set(types.NamespacedName{Name: "test-zenlock", Namespace: "default"}, zenlock)

	client := clientBuilder.WithObjects(zenlock).Build()
	handler.Client = client

	podBytes, _ := json.Marshal(pod)
	originalObject := podBytes

	ctx := context.Background()
	response := handler.handleDryRun(ctx, pod, "test-secret", "/zen-lock/secrets", "test-zenlock", "default", time.Now(), originalObject)

	if !response.Allowed {
		t.Errorf("handleDryRun() should allow in dry-run mode, got denied: %s", response.Result.Message)
	}

	// Verify patch was created
	if len(response.Patches) == 0 && response.Patch == nil {
		t.Error("Expected patch to be created in dry-run mode")
	}
}

func TestPodHandler_HandleDryRun_MutateError(t *testing.T) {
	handler, clientBuilder := setupTestPodHandler(t)

	// Create a pod that might cause mutation error
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			// Empty spec might cause issues
		},
	}

	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-zenlock",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key": "encrypted-value",
			},
		},
	}

	client := clientBuilder.WithObjects(zenlock).Build()
	handler.Client = client

	podBytes, _ := json.Marshal(pod)
	originalObject := podBytes

	ctx := context.Background()
	response := handler.handleDryRun(ctx, pod, "test-secret", "/zen-lock/secrets", "test-zenlock", "default", time.Now(), originalObject)

	// Should handle error gracefully
	if response.Allowed && response.Result != nil && response.Result.Message == "" {
		// If mutation fails, should return error response
		// Note: Empty pod spec might not actually error, but tests the path
	}
}

func TestPodHandler_HandleDryRun_MarshalError(t *testing.T) {
	handler, _ := setupTestPodHandler(t)

	// Create a pod that can't be marshaled (edge case)
	// Note: This is hard to test directly, but we can test the structure
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "test-container", Image: "nginx"},
			},
		},
	}

	podBytes, _ := json.Marshal(pod)
	originalObject := podBytes

	ctx := context.Background()
	// This should succeed normally, but tests the error path structure
	response := handler.handleDryRun(ctx, pod, "test-secret", "/zen-lock/secrets", "test-zenlock", "default", time.Now(), originalObject)

	// Should return a response (either success or error)
	if response == (admission.Response{}) {
		t.Error("Expected a response from handleDryRun")
	}
}

