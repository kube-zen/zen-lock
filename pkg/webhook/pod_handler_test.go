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
	"fmt"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.zen.io/v1alpha1"
)

func setupTestPodHandler(t *testing.T) (*PodHandler, *fake.ClientBuilder) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(securityv1alpha1.AddToScheme(scheme))

	clientBuilder := fake.NewClientBuilder().WithScheme(scheme)

	// Create handler with test private key (this is a test key, not for production)
	privateKey := "AGE-SECRET-1EXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLE"
	handler := &PodHandler{
		Client:     clientBuilder.Build(),
		decoder:    admission.NewDecoder(scheme),
		crypto:     nil, // Will be set if needed
		privateKey: privateKey,
	}

	return handler, clientBuilder
}

func TestPodHandler_Handle_NoInjectionAnnotation(t *testing.T) {
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
		t.Errorf("Expected request to be allowed when no injection annotation, got: %v", resp.Result)
	}
}

func TestValidateAllowedSubjects(t *testing.T) {
	handler, _ := setupTestPodHandler(t)
	ctx := context.Background()

	tests := []struct {
		name            string
		pod             *corev1.Pod
		allowedSubjects []securityv1alpha1.SubjectReference
		wantErr         bool
	}{
		{
			name: "allowed ServiceAccount",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "backend-app",
				},
			},
			allowedSubjects: []securityv1alpha1.SubjectReference{
				{
					Kind:      "ServiceAccount",
					Name:      "backend-app",
					Namespace: "default",
				},
			},
			wantErr: false,
		},
		{
			name: "not allowed ServiceAccount",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "other-app",
				},
			},
			allowedSubjects: []securityv1alpha1.SubjectReference{
				{
					Kind:      "ServiceAccount",
					Name:      "backend-app",
					Namespace: "default",
				},
			},
			wantErr: true,
		},
		{
			name: "default ServiceAccount",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "", // Will default to "default"
				},
			},
			allowedSubjects: []securityv1alpha1.SubjectReference{
				{
					Kind:      "ServiceAccount",
					Name:      "default",
					Namespace: "default",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.validateAllowedSubjects(ctx, tt.pod, tt.allowedSubjects)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAllowedSubjects() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreatePatch(t *testing.T) {
	handler, _ := setupTestPodHandler(t)

	tests := []struct {
		name       string
		pod        *corev1.Pod
		secretName string
		mountPath  string
		wantErr    bool
	}{
		{
			name: "pod with no volumes",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "test-container", Image: "nginx"},
					},
				},
			},
			secretName: "test-secret",
			mountPath:  "/zen-secrets",
			wantErr:    false,
		},
		{
			name: "pod with existing volumes",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{Name: "existing-volume"},
					},
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "nginx",
							VolumeMounts: []corev1.VolumeMount{
								{Name: "existing-volume", MountPath: "/existing"},
							},
						},
					},
				},
			},
			secretName: "test-secret",
			mountPath:  "/zen-secrets",
			wantErr:    false,
		},
		{
			name: "pod with init containers",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{Name: "init-container", Image: "busybox"},
					},
					Containers: []corev1.Container{
						{Name: "test-container", Image: "nginx"},
					},
				},
			},
			secretName: "test-secret",
			mountPath:  "/zen-secrets",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch, err := handler.createPatch(tt.pod, tt.secretName, tt.mountPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("createPatch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(patch) == 0 {
					t.Error("createPatch() returned empty patch")
				}
				// Verify it's valid JSON
				var patches []map[string]interface{}
				if err := json.Unmarshal(patch, &patches); err != nil {
					t.Errorf("createPatch() returned invalid JSON: %v", err)
				}
			}
		})
	}
}

func TestPodHandler_Handle_ZenLockNotFound(t *testing.T) {
	handler, _ := setupTestPodHandler(t)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				"zen-lock/inject": "non-existent-secret",
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

func TestPodHandler_Handle_AllowedSubjectsDenied(t *testing.T) {
	handler, clientBuilder := setupTestPodHandler(t)

	// Create ServiceAccount
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allowed-sa",
			Namespace: "default",
		},
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
					Name:      "allowed-sa",
					Namespace: "default",
				},
			},
		},
	}

	handler.Client = clientBuilder.WithObjects(sa, zenlock).Build()

	// Create Pod with disallowed ServiceAccount
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				"zen-lock/inject": "test-secret",
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "disallowed-sa",
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
		t.Error("Expected request to be denied when ServiceAccount not in AllowedSubjects")
	}
}

func TestPodHandler_Handle_CustomMountPath(t *testing.T) {
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

	patch, err := handler.createPatch(pod, "test-secret", "/custom/path")
	if err != nil {
		t.Fatalf("createPatch() error = %v", err)
	}

	// Verify patch contains custom mount path
	var patches []map[string]interface{}
	if err := json.Unmarshal(patch, &patches); err != nil {
		t.Fatalf("Failed to unmarshal patch: %v", err)
	}

	foundMountPath := false
	for _, p := range patches {
		if path, ok := p["path"].(string); ok && path == "/spec/containers/0/volumeMounts" {
			if value, ok := p["value"].([]interface{}); ok {
				for _, vm := range value {
					if vmMap, ok := vm.(map[string]interface{}); ok {
						if mp, ok := vmMap["mountPath"].(string); ok && mp == "/custom/path" {
							foundMountPath = true
							break
						}
					}
				}
			}
		}
	}

	if !foundMountPath {
		t.Error("Expected patch to contain custom mount path")
	}
}

func TestPodHandler_Handle_MultipleContainers(t *testing.T) {
	handler, _ := setupTestPodHandler(t)

	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "container1", Image: "nginx"},
				{Name: "container2", Image: "busybox"},
				{Name: "container3", Image: "alpine"},
			},
		},
	}

	patch, err := handler.createPatch(pod, "test-secret", "/zen-secrets")
	if err != nil {
		t.Fatalf("createPatch() error = %v", err)
	}

	// Verify patch adds volume mounts to all containers
	var patches []map[string]interface{}
	if err := json.Unmarshal(patch, &patches); err != nil {
		t.Fatalf("Failed to unmarshal patch: %v", err)
	}

	containerMounts := make(map[int]bool)
	for _, p := range patches {
		if path, ok := p["path"].(string); ok {
			// Check for container volume mount paths
			for i := 0; i < 3; i++ {
				expectedPath := fmt.Sprintf("/spec/containers/%d/volumeMounts", i)
				if path == expectedPath {
					containerMounts[i] = true
				}
			}
		}
	}

	if len(containerMounts) != 3 {
		t.Errorf("Expected volume mounts for all 3 containers, got %d", len(containerMounts))
	}
}
