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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kube-zen/zen-lock/pkg/common"
	"github.com/kube-zen/zen-lock/pkg/config"
	"github.com/kube-zen/zen-sdk/pkg/retry"
)

func TestPodHandler_SecretDataMatches(t *testing.T) {
	handler, _ := setupTestPodHandler(t)

	tests := []struct {
		name     string
		existing map[string][]byte
		expected map[string][]byte
		want     bool
	}{
		{
			name:     "empty maps",
			existing: map[string][]byte{},
			expected: map[string][]byte{},
			want:     true,
		},
		{
			name: "matching maps",
			existing: map[string][]byte{
				"key1": []byte("value1"),
				"key2": []byte("value2"),
			},
			expected: map[string][]byte{
				"key1": []byte("value1"),
				"key2": []byte("value2"),
			},
			want: true,
		},
		{
			name: "different lengths",
			existing: map[string][]byte{
				"key1": []byte("value1"),
			},
			expected: map[string][]byte{
				"key1": []byte("value1"),
				"key2": []byte("value2"),
			},
			want: false,
		},
		{
			name: "different values",
			existing: map[string][]byte{
				"key1": []byte("value1"),
			},
			expected: map[string][]byte{
				"key1": []byte("value2"),
			},
			want: false,
		},
		{
			name: "missing key",
			existing: map[string][]byte{
				"key1": []byte("value1"),
			},
			expected: map[string][]byte{
				"key2": []byte("value2"),
			},
			want: false,
		},
		{
			name:     "nil existing",
			existing: nil,
			expected: map[string][]byte{
				"key1": []byte("value1"),
			},
			want: false,
		},
		{
			name: "nil expected",
			existing: map[string][]byte{
				"key1": []byte("value1"),
			},
			expected: nil,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handler.secretDataMatches(tt.existing, tt.expected)
			if got != tt.want {
				t.Errorf("secretDataMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPodHandler_EnsureSecretExists_CreateNew(t *testing.T) {
	handler, clientBuilder := setupTestPodHandler(t)

	secretName := "test-secret"
	secretData := map[string][]byte{
		"key": []byte("value"),
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "default",
			Labels: map[string]string{
				common.LabelZenLockName:  "test-zenlock",
				common.LabelPodName:      "test-pod",
				common.LabelPodNamespace: "default",
			},
		},
		Data: secretData,
	}

	client := clientBuilder.Build()
	handler.Client = client

	ctx := context.Background()
	retryConfig := retry.DefaultConfig()
	retryConfig.MaxAttempts = config.DefaultRetryMaxAttempts
	retryConfig.InitialDelay = config.DefaultRetryInitialDelay
	retryConfig.MaxDelay = config.DefaultRetryMaxDelay

	err := handler.ensureSecretExists(ctx, secret, secretName, "test-zenlock", "default", "test-pod", secretData, time.Now(), retryConfig, false)
	if err != nil {
		t.Errorf("ensureSecretExists() error = %v, want no error", err)
	}

	// Verify secret was created
	createdSecret := &corev1.Secret{}
	if err := client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: "default"}, createdSecret); err != nil {
		t.Fatalf("Failed to get created secret: %v", err)
	}

	if len(createdSecret.Data) != 1 {
		t.Errorf("Expected 1 data key, got %d", len(createdSecret.Data))
	}
}

func TestPodHandler_EnsureSecretExists_UpdateStaleData(t *testing.T) {
	handler, clientBuilder := setupTestPodHandler(t)

	secretName := "test-secret"
	oldData := map[string][]byte{
		"key": []byte("old-value"),
	}
	newData := map[string][]byte{
		"key": []byte("new-value"),
	}

	// Create existing secret with old data
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "default",
			Labels: map[string]string{
				common.LabelZenLockName:  "test-zenlock",
				common.LabelPodName:      "test-pod",
				common.LabelPodNamespace: "default",
			},
		},
		Data: oldData,
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "default",
			Labels: map[string]string{
				common.LabelZenLockName:  "test-zenlock",
				common.LabelPodName:      "test-pod",
				common.LabelPodNamespace: "default",
			},
		},
		Data: newData,
	}

	client := clientBuilder.WithObjects(existingSecret).Build()
	handler.Client = client

	ctx := context.Background()
	retryConfig := retry.DefaultConfig()
	retryConfig.MaxAttempts = config.DefaultRetryMaxAttempts
	retryConfig.InitialDelay = config.DefaultRetryInitialDelay
	retryConfig.MaxDelay = config.DefaultRetryMaxDelay

	err := handler.ensureSecretExists(ctx, secret, secretName, "test-zenlock", "default", "test-pod", newData, time.Now(), retryConfig, false)
	if err != nil {
		t.Errorf("ensureSecretExists() error = %v, want no error", err)
	}

	// Verify secret was updated
	updatedSecret := &corev1.Secret{}
	if err := client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: "default"}, updatedSecret); err != nil {
		t.Fatalf("Failed to get updated secret: %v", err)
	}

	if string(updatedSecret.Data["key"]) != "new-value" {
		t.Errorf("Expected secret data 'new-value', got '%s'", string(updatedSecret.Data["key"]))
	}
}

func TestPodHandler_EnsureSecretExists_UpdateDifferentZenLock(t *testing.T) {
	handler, clientBuilder := setupTestPodHandler(t)

	secretName := "test-secret"
	secretData := map[string][]byte{
		"key": []byte("value"),
	}

	// Create existing secret for different ZenLock
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "default",
			Labels: map[string]string{
				common.LabelZenLockName:  "old-zenlock",
				common.LabelPodName:      "test-pod",
				common.LabelPodNamespace: "default",
			},
		},
		Data: map[string][]byte{
			"key": []byte("old-value"),
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "default",
			Labels: map[string]string{
				common.LabelZenLockName:  "new-zenlock",
				common.LabelPodName:      "test-pod",
				common.LabelPodNamespace: "default",
			},
		},
		Data: secretData,
	}

	client := clientBuilder.WithObjects(existingSecret).Build()
	handler.Client = client

	ctx := context.Background()
	retryConfig := retry.DefaultConfig()
	retryConfig.MaxAttempts = config.DefaultRetryMaxAttempts
	retryConfig.InitialDelay = config.DefaultRetryInitialDelay
	retryConfig.MaxDelay = config.DefaultRetryMaxDelay

	err := handler.ensureSecretExists(ctx, secret, secretName, "new-zenlock", "default", "test-pod", secretData, time.Now(), retryConfig, false)
	if err != nil {
		t.Errorf("ensureSecretExists() error = %v, want no error", err)
	}

	// Verify secret was updated with new ZenLock name
	updatedSecret := &corev1.Secret{}
	if err := client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: "default"}, updatedSecret); err != nil {
		t.Fatalf("Failed to get updated secret: %v", err)
	}

	if updatedSecret.Labels[common.LabelZenLockName] != "new-zenlock" {
		t.Errorf("Expected ZenLock name 'new-zenlock', got '%s'", updatedSecret.Labels[common.LabelZenLockName])
	}
}

func TestPodHandler_EnsureSecretExists_DryRun(t *testing.T) {
	handler, clientBuilder := setupTestPodHandler(t)

	secretName := "test-secret"
	secretData := map[string][]byte{
		"key": []byte("value"),
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "default",
			Labels: map[string]string{
				common.LabelZenLockName:  "test-zenlock",
				common.LabelPodName:      "test-pod",
				common.LabelPodNamespace: "default",
			},
		},
		Data: secretData,
	}

	client := clientBuilder.Build()
	handler.Client = client

	ctx := context.Background()
	retryConfig := retry.DefaultConfig()
	retryConfig.MaxAttempts = config.DefaultRetryMaxAttempts
	retryConfig.InitialDelay = config.DefaultRetryInitialDelay
	retryConfig.MaxDelay = config.DefaultRetryMaxDelay

	// Test with isDryRun = true
	err := handler.ensureSecretExists(ctx, secret, secretName, "test-zenlock", "default", "test-pod", secretData, time.Now(), retryConfig, true)
	if err != nil {
		t.Errorf("ensureSecretExists() error = %v, want no error", err)
	}

	// Verify secret was NOT created in dry-run mode
	createdSecret := &corev1.Secret{}
	if err := client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: "default"}, createdSecret); err == nil {
		t.Error("Expected secret to not be created in dry-run mode")
	}
}

func TestPodHandler_EnsureSecretExists_ExistingSecretMatches(t *testing.T) {
	handler, clientBuilder := setupTestPodHandler(t)

	secretName := "test-secret"
	secretData := map[string][]byte{
		"key": []byte("value"),
	}

	// Create existing secret with matching data
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "default",
			Labels: map[string]string{
				common.LabelZenLockName:  "test-zenlock",
				common.LabelPodName:      "test-pod",
				common.LabelPodNamespace: "default",
			},
		},
		Data: secretData,
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "default",
			Labels: map[string]string{
				common.LabelZenLockName:  "test-zenlock",
				common.LabelPodName:      "test-pod",
				common.LabelPodNamespace: "default",
			},
		},
		Data: secretData,
	}

	client := clientBuilder.WithObjects(existingSecret).Build()
	handler.Client = client

	ctx := context.Background()
	retryConfig := retry.DefaultConfig()
	retryConfig.MaxAttempts = config.DefaultRetryMaxAttempts
	retryConfig.InitialDelay = config.DefaultRetryInitialDelay
	retryConfig.MaxDelay = config.DefaultRetryMaxDelay

	err := handler.ensureSecretExists(ctx, secret, secretName, "test-zenlock", "default", "test-pod", secretData, time.Now(), retryConfig, false)
	if err != nil {
		t.Errorf("ensureSecretExists() error = %v, want no error", err)
	}

	// Verify secret still exists and wasn't unnecessarily updated
	updatedSecret := &corev1.Secret{}
	if err := client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: "default"}, updatedSecret); err != nil {
		t.Fatalf("Failed to get secret: %v", err)
	}

	if string(updatedSecret.Data["key"]) != "value" {
		t.Errorf("Expected secret data 'value', got '%s'", string(updatedSecret.Data["key"]))
	}
}

func TestPodHandler_EnsureSecretExists_NilLabels(t *testing.T) {
	handler, clientBuilder := setupTestPodHandler(t)

	secretName := "test-secret"
	secretData := map[string][]byte{
		"key": []byte("value"),
	}

	// Create existing secret with nil labels
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "default",
			Labels:    nil, // nil labels
		},
		Data: secretData,
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "default",
			Labels: map[string]string{
				common.LabelZenLockName:  "test-zenlock",
				common.LabelPodName:      "test-pod",
				common.LabelPodNamespace: "default",
			},
		},
		Data: secretData,
	}

	client := clientBuilder.WithObjects(existingSecret).Build()
	handler.Client = client

	ctx := context.Background()
	retryConfig := retry.DefaultConfig()
	retryConfig.MaxAttempts = config.DefaultRetryMaxAttempts
	retryConfig.InitialDelay = config.DefaultRetryInitialDelay
	retryConfig.MaxDelay = config.DefaultRetryMaxDelay

	err := handler.ensureSecretExists(ctx, secret, secretName, "test-zenlock", "default", "test-pod", secretData, time.Now(), retryConfig, false)
	if err != nil {
		t.Errorf("ensureSecretExists() error = %v, want no error", err)
	}

	// Verify secret was updated with labels
	updatedSecret := &corev1.Secret{}
	if err := client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: "default"}, updatedSecret); err != nil {
		t.Fatalf("Failed to get updated secret: %v", err)
	}

	if updatedSecret.Labels == nil {
		t.Error("Expected labels to be initialized")
	}
	if updatedSecret.Labels[common.LabelZenLockName] != "test-zenlock" {
		t.Errorf("Expected ZenLock name 'test-zenlock', got '%s'", updatedSecret.Labels[common.LabelZenLockName])
	}
}
