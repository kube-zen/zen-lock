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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPodHandler_MutatePod(t *testing.T) {
	handler, _ := setupTestPodHandler(t)

	tests := []struct {
		name       string
		pod        *corev1.Pod
		secretName string
		mountPath  string
		wantErr    bool
		validate   func(t *testing.T, pod *corev1.Pod)
	}{
		{
			name: "pod with no volumes",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "test-container", Image: "nginx"},
					},
				},
			},
			secretName: "test-secret",
			mountPath:  "/zen-lock/secrets",
			wantErr:    false,
			validate: func(t *testing.T, pod *corev1.Pod) {
				if len(pod.Spec.Volumes) != 1 {
					t.Errorf("Expected 1 volume, got %d", len(pod.Spec.Volumes))
				}
				if pod.Spec.Volumes[0].Name != "zen-secrets" {
					t.Errorf("Expected volume name 'zen-secrets', got '%s'", pod.Spec.Volumes[0].Name)
				}
				if pod.Spec.Volumes[0].Secret.SecretName != "test-secret" {
					t.Errorf("Expected secret name 'test-secret', got '%s'", pod.Spec.Volumes[0].Secret.SecretName)
				}
				if len(pod.Spec.Containers[0].VolumeMounts) != 1 {
					t.Errorf("Expected 1 volume mount, got %d", len(pod.Spec.Containers[0].VolumeMounts))
				}
				if pod.Spec.Containers[0].VolumeMounts[0].MountPath != "/zen-lock/secrets" {
					t.Errorf("Expected mount path '/zen-lock/secrets', got '%s'", pod.Spec.Containers[0].VolumeMounts[0].MountPath)
				}
			},
		},
		{
			name: "pod with existing volumes",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{Name: "existing-volume", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
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
			mountPath:  "/zen-lock/secrets",
			wantErr:    false,
			validate: func(t *testing.T, pod *corev1.Pod) {
				if len(pod.Spec.Volumes) != 2 {
					t.Errorf("Expected 2 volumes, got %d", len(pod.Spec.Volumes))
				}
				if len(pod.Spec.Containers[0].VolumeMounts) != 2 {
					t.Errorf("Expected 2 volume mounts, got %d", len(pod.Spec.Containers[0].VolumeMounts))
				}
			},
		},
		{
			name: "pod with multiple containers",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "container1", Image: "nginx"},
						{Name: "container2", Image: "busybox"},
					},
				},
			},
			secretName: "test-secret",
			mountPath:  "/zen-lock/secrets",
			wantErr:    false,
			validate: func(t *testing.T, pod *corev1.Pod) {
				if len(pod.Spec.Volumes) != 1 {
					t.Errorf("Expected 1 volume, got %d", len(pod.Spec.Volumes))
				}
				// All containers should have the volume mount
				for i, container := range pod.Spec.Containers {
					if len(container.VolumeMounts) != 1 {
						t.Errorf("Container %d: Expected 1 volume mount, got %d", i, len(container.VolumeMounts))
					}
				}
			},
		},
		{
			name: "pod with init containers",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
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
			mountPath:  "/zen-lock/secrets",
			wantErr:    false,
			validate: func(t *testing.T, pod *corev1.Pod) {
				if len(pod.Spec.Volumes) != 1 {
					t.Errorf("Expected 1 volume, got %d", len(pod.Spec.Volumes))
				}
				// Init containers should also have the volume mount
				if len(pod.Spec.InitContainers[0].VolumeMounts) != 1 {
					t.Errorf("Init container: Expected 1 volume mount, got %d", len(pod.Spec.InitContainers[0].VolumeMounts))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.mutatePod(tt.pod, tt.secretName, tt.mountPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("mutatePod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.validate != nil {
				tt.validate(t, tt.pod)
			}
		})
	}
}

func TestPodHandler_MutatePod_CustomMountPath(t *testing.T) {
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

	customMountPath := "/custom/secrets"
	err := handler.mutatePod(pod, "test-secret", customMountPath)
	if err != nil {
		t.Errorf("mutatePod() error = %v", err)
	}

	if pod.Spec.Containers[0].VolumeMounts[0].MountPath != customMountPath {
		t.Errorf("Expected mount path '%s', got '%s'", customMountPath, pod.Spec.Containers[0].VolumeMounts[0].MountPath)
	}
}
