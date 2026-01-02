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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
	"github.com/kube-zen/zen-lock/pkg/common"
	"github.com/kube-zen/zen-sdk/pkg/lifecycle"
)

func TestZenLockReconciler_Reconcile_Deletion(t *testing.T) {
	reconciler, clientBuilder := setupTestReconciler(t)

	// Add corev1 to scheme for Secret objects
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(securityv1alpha1.AddToScheme(scheme))
	clientBuilder = clientBuilder.WithScheme(scheme)

	now := metav1.Now()
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-secret",
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{zenLockFinalizer},
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": "dGVzdA==",
			},
		},
	}

	// Create associated secrets
	secret1 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret1",
			Namespace: "default",
			Labels: map[string]string{
				common.LabelZenLockName: "test-secret",
			},
		},
		Data: map[string][]byte{
			"key1": []byte("value1"),
		},
	}

	secret2 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret2",
			Namespace: "default",
			Labels: map[string]string{
				common.LabelZenLockName: "test-secret",
			},
		},
		Data: map[string][]byte{
			"key2": []byte("value2"),
		},
	}

	// Secret in different namespace (should not be deleted)
	secret3 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret3",
			Namespace: "other",
			Labels: map[string]string{
				common.LabelZenLockName: "test-secret",
			},
		},
		Data: map[string][]byte{
			"key3": []byte("value3"),
		},
	}

	client := clientBuilder.WithObjects(zenlock, secret1, secret2, secret3).Build()
	reconciler.Client = client

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-secret",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)

	if err != nil {
		t.Errorf("Reconcile() should not error for deletion, got: %v", err)
	}
	if result.RequeueAfter > 0 {
		t.Error("Reconcile() should not requeue after successful deletion")
	}

	// Verify secrets in same namespace were deleted
	secret1Check := &corev1.Secret{}
	if err := client.Get(ctx, types.NamespacedName{Name: "secret1", Namespace: "default"}, secret1Check); err == nil {
		t.Error("Expected secret1 to be deleted")
	}

	secret2Check := &corev1.Secret{}
	if err := client.Get(ctx, types.NamespacedName{Name: "secret2", Namespace: "default"}, secret2Check); err == nil {
		t.Error("Expected secret2 to be deleted")
	}

	// Verify secret in different namespace still exists
	secret3Check := &corev1.Secret{}
	if err := client.Get(ctx, types.NamespacedName{Name: "secret3", Namespace: "other"}, secret3Check); err != nil {
		t.Error("Expected secret3 in different namespace to still exist")
	}

	// Verify finalizer was removed (ZenLock may be deleted by fake client after finalizer removal)
	updatedZenLock := &securityv1alpha1.ZenLock{}
	err = client.Get(ctx, req.NamespacedName, updatedZenLock)
	if err == nil {
		// If ZenLock still exists, verify finalizer is removed
		if lifecycle.ContainsString(updatedZenLock.Finalizers, zenLockFinalizer) {
			t.Error("Expected finalizer to be removed")
		}
	}
	// If ZenLock is not found, that's also acceptable as it may have been deleted
}

func TestZenLockReconciler_Reconcile_Deletion_NoFinalizer(t *testing.T) {
	reconciler, clientBuilder := setupTestReconciler(t)

	// Add corev1 to scheme for Secret objects (even though we don't use Secrets here, keep consistent)
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(securityv1alpha1.AddToScheme(scheme))
	clientBuilder = clientBuilder.WithScheme(scheme)

	// Create ZenLock with deletionTimestamp but no finalizer
	// Fake client requires a finalizer if deletionTimestamp is set, so we add a dummy one
	// then remove it before reconciling
	now := metav1.Now()
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-secret",
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{"dummy-finalizer"}, // Dummy finalizer to satisfy fake client
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": "dGVzdA==",
			},
		},
	}

	client := clientBuilder.WithObjects(zenlock).Build()
	reconciler.Client = client

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-secret",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	// Remove the dummy finalizer so the ZenLock has deletionTimestamp but no finalizer
	zenlock.Finalizers = []string{}
	if err := client.Update(ctx, zenlock); err != nil {
		t.Fatalf("Failed to remove finalizer: %v", err)
	}
	result, err := reconciler.Reconcile(ctx, req)

	if err != nil {
		t.Errorf("Reconcile() should not error when no finalizer, got: %v", err)
	}
	if result.RequeueAfter > 0 {
		t.Error("Reconcile() should not requeue when no finalizer")
	}
}

func TestRemoveString(t *testing.T) {
	testCases := []struct {
		name     string
		slice    []string
		s        string
		expected []string
	}{
		{
			name:     "remove from middle",
			slice:    []string{"a", "b", "c", "d"},
			s:        "b",
			expected: []string{"a", "c", "d"},
		},
		{
			name:     "remove from beginning",
			slice:    []string{"a", "b", "c"},
			s:        "a",
			expected: []string{"b", "c"},
		},
		{
			name:     "remove from end",
			slice:    []string{"a", "b", "c"},
			s:        "c",
			expected: []string{"a", "b"},
		},
		{
			name:     "remove non-existent",
			slice:    []string{"a", "b", "c"},
			s:        "d",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "remove multiple occurrences",
			slice:    []string{"a", "b", "a", "c", "a"},
			s:        "a",
			expected: []string{"b", "c"},
		},
		{
			name:     "empty slice",
			slice:    []string{},
			s:        "a",
			expected: []string{},
		},
		{
			name:     "single element match",
			slice:    []string{"a"},
			s:        "a",
			expected: []string{},
		},
		{
			name:     "single element no match",
			slice:    []string{"a"},
			s:        "b",
			expected: []string{"a"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := lifecycle.RemoveString(tc.slice, tc.s)
			if len(result) != len(tc.expected) {
				t.Errorf("Expected length %d, got %d", len(tc.expected), len(result))
				return
			}
			for i, v := range result {
				if v != tc.expected[i] {
					t.Errorf("Expected %v at index %d, got %v", tc.expected[i], i, v)
				}
			}
		})
	}
}
