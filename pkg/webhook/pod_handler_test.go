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
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.zen.io/v1alpha1"
	"github.com/kube-zen/zen-lock/pkg/crypto"
)

func setupTestPodHandler(t *testing.T) (*PodHandler, *fake.ClientBuilder) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(securityv1alpha1.AddToScheme(scheme))

	clientBuilder := fake.NewClientBuilder().WithScheme(scheme)

	// Create handler with test private key (this is a test key, not for production)
	privateKey := "AGE-SECRET-1EXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLE"
	
	// Initialize crypto (needed for decryption)
	encryptor := crypto.NewAgeEncryptor()
	
	// Initialize cache for tests
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

func TestMutatePod(t *testing.T) {
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
			originalPod := tt.pod.DeepCopy()
			err := handler.mutatePod(tt.pod, tt.secretName, tt.mountPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("mutatePod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Verify volume was added
				foundVolume := false
				for _, vol := range tt.pod.Spec.Volumes {
					if vol.Name == "zen-secrets" && vol.Secret != nil && vol.Secret.SecretName == tt.secretName {
						foundVolume = true
						break
					}
				}
				if !foundVolume {
					t.Error("mutatePod() did not add expected volume")
				}

				// Verify volume mounts were added to containers
				for i, container := range tt.pod.Spec.Containers {
					foundMount := false
					for _, mount := range container.VolumeMounts {
						if mount.Name == "zen-secrets" && mount.MountPath == tt.mountPath {
							foundMount = true
							break
						}
					}
					if !foundMount {
						t.Errorf("mutatePod() did not add volume mount to container %d", i)
					}
				}

				// Verify original pod was not modified (deep copy worked)
				if len(originalPod.Spec.Volumes) == len(tt.pod.Spec.Volumes) && len(originalPod.Spec.Volumes) > 0 {
					t.Error("mutatePod() should have added a volume")
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

	err := handler.mutatePod(pod, "test-secret", "/custom/path")
	if err != nil {
		t.Fatalf("mutatePod() error = %v", err)
	}

	// Verify custom mount path was set
	foundMountPath := false
	for _, container := range pod.Spec.Containers {
		for _, mount := range container.VolumeMounts {
			if mount.Name == "zen-secrets" && mount.MountPath == "/custom/path" {
				foundMountPath = true
				break
			}
		}
	}

	if !foundMountPath {
		t.Error("Expected mutatePod to set custom mount path")
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

	err := handler.mutatePod(pod, "test-secret", "/zen-secrets")
	if err != nil {
		t.Fatalf("mutatePod() error = %v", err)
	}

	// Verify volume mounts were added to all containers
	if len(pod.Spec.Containers) != 3 {
		t.Fatalf("Expected 3 containers, got %d", len(pod.Spec.Containers))
	}

	for i, container := range pod.Spec.Containers {
		foundMount := false
		for _, mount := range container.VolumeMounts {
			if mount.Name == "zen-secrets" {
				foundMount = true
				break
			}
		}
		if !foundMount {
			t.Errorf("Expected volume mount for container %d (%s), but not found", i, container.Name)
		}
	}
}
