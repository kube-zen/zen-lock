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

package controller

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func setupSecretReconciler(t *testing.T) (*SecretReconciler, *fake.ClientBuilder) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)

	clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
	reconciler := NewSecretReconciler(clientBuilder.Build(), scheme)

	return reconciler, clientBuilder
}

func TestSecretReconciler_IgnoresNonZenLockSecrets(t *testing.T) {
	reconciler, clientBuilder := setupSecretReconciler(t)

	// Create a regular Secret without zen-lock labels
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "regular-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"key": []byte("value"),
		},
	}

	client := clientBuilder.WithObjects(secret).Build()
	reconciler.Client = client

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "regular-secret",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)

	if err != nil {
		t.Errorf("Reconcile() error = %v, want no error", err)
	}

	if result.Requeue {
		t.Error("Reconcile() should not requeue for non-zen-lock secrets")
	}
}

func TestSecretReconciler_SetsOwnerReferenceWhenPodExists(t *testing.T) {
	reconciler, clientBuilder := setupSecretReconciler(t)

	// Create Pod with UID
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			UID:       types.UID("test-pod-uid-123"),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "test-container", Image: "nginx"},
			},
		},
	}

	// Create Secret with zen-lock labels but no OwnerReference
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zen-lock-secret",
			Namespace: "default",
			Labels: map[string]string{
				labelPodName:      "test-pod",
				labelPodNamespace: "default",
				labelZenLockName:  "test-zenlock",
			},
		},
		Data: map[string][]byte{
			"key": []byte("value"),
		},
	}

	client := clientBuilder.WithObjects(pod, secret).Build()
	reconciler.Client = client

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "zen-lock-secret",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)

	if err != nil {
		t.Errorf("Reconcile() error = %v", err)
	}

	if result.Requeue {
		t.Error("Reconcile() should not requeue when OwnerReference is set")
	}

	// Verify OwnerReference was set
	updatedSecret := &corev1.Secret{}
	if err := client.Get(ctx, req.NamespacedName, updatedSecret); err != nil {
		t.Fatalf("Failed to get updated Secret: %v", err)
	}

	if len(updatedSecret.OwnerReferences) == 0 {
		t.Error("Expected OwnerReference to be set")
	} else {
		ownerRef := updatedSecret.OwnerReferences[0]
		if ownerRef.Kind != "Pod" {
			t.Errorf("Expected OwnerReference kind 'Pod', got '%s'", ownerRef.Kind)
		}
		if ownerRef.Name != "test-pod" {
			t.Errorf("Expected OwnerReference name 'test-pod', got '%s'", ownerRef.Name)
		}
		if ownerRef.UID != pod.UID {
			t.Errorf("Expected OwnerReference UID %s, got %s", pod.UID, ownerRef.UID)
		}
		if ownerRef.Controller == nil || !*ownerRef.Controller {
			t.Error("Expected OwnerReference Controller to be true")
		}
	}
}

func TestSecretReconciler_RequeuesWhenPodNotExists(t *testing.T) {
	reconciler, clientBuilder := setupSecretReconciler(t)

	// Create Secret with zen-lock labels but Pod doesn't exist
	// Use a recent timestamp so it won't be deleted as orphaned
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zen-lock-secret",
			Namespace: "default",
			Labels: map[string]string{
				labelPodName:      "non-existent-pod",
				labelPodNamespace: "default",
				labelZenLockName:  "test-zenlock",
			},
			CreationTimestamp: metav1.Now(), // Recent timestamp
		},
		Data: map[string][]byte{
			"key": []byte("value"),
		},
	}

	client := clientBuilder.WithObjects(secret).Build()
	reconciler.Client = client

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "zen-lock-secret",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)

	if err != nil {
		t.Errorf("Reconcile() error = %v", err)
	}

	// When Pod doesn't exist and Secret is recent, reconciler requeues with a delay
	if result.RequeueAfter == 0 {
		t.Error("Reconcile() should set RequeueAfter when Pod doesn't exist")
	}

	// RequeueAfter should be 5 seconds (as per implementation)
	expectedDelay := 5 * time.Second
	if result.RequeueAfter != expectedDelay {
		t.Errorf("Reconcile() RequeueAfter = %v, want %v", result.RequeueAfter, expectedDelay)
	}
}

func TestSecretReconciler_DeletesOrphanedSecret(t *testing.T) {
	reconciler, clientBuilder := setupSecretReconciler(t)

	// Create Secret with zen-lock labels but Pod doesn't exist
	// Use an old timestamp so it will be deleted as orphaned (older than default 15min TTL)
	oldTime := metav1.NewTime(time.Now().Add(-20 * time.Minute)) // 20 minutes ago
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zen-lock-secret",
			Namespace: "default",
			Labels: map[string]string{
				labelPodName:      "non-existent-pod",
				labelPodNamespace: "default",
				labelZenLockName:  "test-zenlock",
			},
			CreationTimestamp: oldTime, // Old timestamp
		},
		Data: map[string][]byte{
			"key": []byte("value"),
		},
	}

	client := clientBuilder.WithObjects(secret).Build()
	reconciler.Client = client

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "zen-lock-secret",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)

	if err != nil {
		t.Errorf("Reconcile() error = %v", err)
	}

	// Should not requeue when deleting orphaned secret
	if result.Requeue {
		t.Error("Reconcile() should not requeue when deleting orphaned secret")
	}

	// Verify Secret was deleted
	updatedSecret := &corev1.Secret{}
	if err := client.Get(ctx, req.NamespacedName, updatedSecret); err == nil {
		t.Error("Expected Secret to be deleted")
	}
}

func TestSecretReconciler_RequeuesWhenPodHasNoUID(t *testing.T) {
	reconciler, clientBuilder := setupSecretReconciler(t)

	// Create Pod without UID
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			// No UID
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "test-container", Image: "nginx"},
			},
		},
	}

	// Create Secret with zen-lock labels
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zen-lock-secret",
			Namespace: "default",
			Labels: map[string]string{
				labelPodName:      "test-pod",
				labelPodNamespace: "default",
				labelZenLockName:  "test-zenlock",
			},
		},
		Data: map[string][]byte{
			"key": []byte("value"),
		},
	}

	client := clientBuilder.WithObjects(pod, secret).Build()
	reconciler.Client = client

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "zen-lock-secret",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)

	if err != nil {
		t.Errorf("Reconcile() error = %v", err)
	}

	// When Pod has no UID, reconciler requeues with a delay
	if result.RequeueAfter == 0 {
		t.Error("Reconcile() should set RequeueAfter when Pod has no UID")
	}

	// RequeueAfter should be 2 seconds (as per implementation)
	expectedDelay := 2 * time.Second
	if result.RequeueAfter != expectedDelay {
		t.Errorf("Reconcile() RequeueAfter = %v, want %v", result.RequeueAfter, expectedDelay)
	}
}

func TestSecretReconciler_SkipsWhenOwnerReferenceExists(t *testing.T) {
	reconciler, clientBuilder := setupSecretReconciler(t)

	// Create Pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			UID:       types.UID("test-pod-uid-123"),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "test-container", Image: "nginx"},
			},
		},
	}

	// Create Secret with zen-lock labels and existing OwnerReference
	controller := true
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zen-lock-secret",
			Namespace: "default",
			Labels: map[string]string{
				labelPodName:      "test-pod",
				labelPodNamespace: "default",
				labelZenLockName:  "test-zenlock",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "test-pod",
					UID:        pod.UID,
					Controller: &controller,
				},
			},
		},
		Data: map[string][]byte{
			"key": []byte("value"),
		},
	}

	client := clientBuilder.WithObjects(pod, secret).Build()
	reconciler.Client = client

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "zen-lock-secret",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)

	if err != nil {
		t.Errorf("Reconcile() error = %v", err)
	}

	if result.Requeue {
		t.Error("Reconcile() should not requeue when OwnerReference already exists")
	}

	// Verify Secret was not modified (no update needed)
	updatedSecret := &corev1.Secret{}
	if err := client.Get(ctx, req.NamespacedName, updatedSecret); err != nil {
		t.Fatalf("Failed to get Secret: %v", err)
	}

	if len(updatedSecret.OwnerReferences) != 1 {
		t.Errorf("Expected 1 OwnerReference, got %d", len(updatedSecret.OwnerReferences))
	}
}
