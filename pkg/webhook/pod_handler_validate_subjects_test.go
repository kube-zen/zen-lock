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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
)

func TestPodHandler_ValidateAllowedSubjects_EmptyServiceAccount(t *testing.T) {
	handler, _ := setupTestPodHandler(t)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			// Empty ServiceAccountName defaults to "default"
		},
	}

	allowedSubjects := []securityv1alpha1.SubjectReference{
		{
			Kind:      "ServiceAccount",
			Name:      "default",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	err := handler.validateAllowedSubjects(ctx, pod, allowedSubjects)
	if err != nil {
		t.Errorf("validateAllowedSubjects() error = %v, want no error for default ServiceAccount", err)
	}
}

func TestPodHandler_ValidateAllowedSubjects_EmptyNamespace(t *testing.T) {
	handler, _ := setupTestPodHandler(t)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
			// Empty namespace defaults to "default"
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "test-sa",
		},
	}

	allowedSubjects := []securityv1alpha1.SubjectReference{
		{
			Kind:      "ServiceAccount",
			Name:      "test-sa",
			Namespace: "", // Empty namespace uses pod namespace
		},
	}

	ctx := context.Background()
	err := handler.validateAllowedSubjects(ctx, pod, allowedSubjects)
	if err != nil {
		t.Errorf("validateAllowedSubjects() error = %v, want no error when subject namespace is empty", err)
	}
}

func TestPodHandler_ValidateAllowedSubjects_NonServiceAccountKind(t *testing.T) {
	handler, _ := setupTestPodHandler(t)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "test-sa",
		},
	}

	allowedSubjects := []securityv1alpha1.SubjectReference{
		{
			Kind:      "User", // Not ServiceAccount
			Name:      "test-user",
			Namespace: "default",
		},
		{
			Kind:      "Group", // Not ServiceAccount
			Name:      "test-group",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	err := handler.validateAllowedSubjects(ctx, pod, allowedSubjects)
	if err == nil {
		t.Error("validateAllowedSubjects() should return error when no ServiceAccount matches")
	}
}

func TestPodHandler_ValidateAllowedSubjects_NoMatch(t *testing.T) {
	handler, _ := setupTestPodHandler(t)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "test-sa",
		},
	}

	allowedSubjects := []securityv1alpha1.SubjectReference{
		{
			Kind:      "ServiceAccount",
			Name:      "other-sa",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	err := handler.validateAllowedSubjects(ctx, pod, allowedSubjects)
	if err == nil {
		t.Error("validateAllowedSubjects() should return error when ServiceAccount doesn't match")
	}
}

func TestPodHandler_ValidateAllowedSubjects_DifferentNamespace(t *testing.T) {
	handler, _ := setupTestPodHandler(t)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "test-sa",
		},
	}

	allowedSubjects := []securityv1alpha1.SubjectReference{
		{
			Kind:      "ServiceAccount",
			Name:      "test-sa",
			Namespace: "other-namespace", // Different namespace
		},
	}

	ctx := context.Background()
	err := handler.validateAllowedSubjects(ctx, pod, allowedSubjects)
	if err == nil {
		t.Error("validateAllowedSubjects() should return error when namespaces don't match")
	}
}

func TestPodHandler_ValidateAllowedSubjects_MultipleSubjects(t *testing.T) {
	handler, _ := setupTestPodHandler(t)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "test-sa",
		},
	}

	allowedSubjects := []securityv1alpha1.SubjectReference{
		{
			Kind:      "ServiceAccount",
			Name:      "other-sa",
			Namespace: "default",
		},
		{
			Kind:      "ServiceAccount",
			Name:      "test-sa", // This one matches
			Namespace: "default",
		},
	}

	ctx := context.Background()
	err := handler.validateAllowedSubjects(ctx, pod, allowedSubjects)
	if err != nil {
		t.Errorf("validateAllowedSubjects() error = %v, want no error when one subject matches", err)
	}
}

func TestPodHandler_ValidateAllowedSubjects_EmptyList(t *testing.T) {
	handler, _ := setupTestPodHandler(t)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "test-sa",
		},
	}

	allowedSubjects := []securityv1alpha1.SubjectReference{}

	ctx := context.Background()
	err := handler.validateAllowedSubjects(ctx, pod, allowedSubjects)
	if err == nil {
		t.Error("validateAllowedSubjects() should return error when allowedSubjects is empty")
	}
}

