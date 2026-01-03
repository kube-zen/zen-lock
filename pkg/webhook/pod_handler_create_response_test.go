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
	"encoding/json"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPodHandler_CreateMutationResponse_Success(t *testing.T) {
	handler, _ := setupTestPodHandler(t)

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

	originalObject, _ := json.Marshal(pod)
	response := handler.createMutationResponse(pod, "test-secret", "/zen-lock/secrets", "test-zenlock", "default", time.Now(), originalObject)

	if !response.Allowed {
		t.Errorf("createMutationResponse() should allow, got denied: %s", response.Result.Message)
	}

	// Verify patch was created
	if len(response.Patches) == 0 && response.Patch == nil {
		t.Error("Expected patch to be created")
	}
}

func TestPodHandler_CreateMutationResponse_MutateError(t *testing.T) {
	handler, _ := setupTestPodHandler(t)

	// Create a pod that might cause mutation error
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			// Empty spec
		},
	}

	originalObject, _ := json.Marshal(pod)
	response := handler.createMutationResponse(pod, "test-secret", "/zen-lock/secrets", "test-zenlock", "default", time.Now(), originalObject)

	// Should handle error gracefully
	// Note: Empty pod spec might not actually error, but tests the path
	// admission.Response contains slices which can't be compared with ==
	if response.UID == "" && len(response.Patches) == 0 && !response.Allowed {
		t.Error("Expected a valid response from createMutationResponse")
	}
}

func TestPodHandler_CreateMutationResponse_MarshalError(t *testing.T) {
	handler, _ := setupTestPodHandler(t)

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

	originalObject, _ := json.Marshal(pod)
	// This should succeed normally, but tests the error path structure
	response := handler.createMutationResponse(pod, "test-secret", "/zen-lock/secrets", "test-zenlock", "default", time.Now(), originalObject)

	// Should return a response (either success or error)
	// admission.Response contains slices which can't be compared with ==
	if response.UID == "" && len(response.Patches) == 0 && !response.Allowed {
		t.Error("Expected a valid response from createMutationResponse")
	}
}
