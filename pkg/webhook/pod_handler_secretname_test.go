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
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestGenerateSecretName(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		podName   string
		validate  func(t *testing.T, secretName string)
	}{
		{
			name:      "normal names",
			namespace: "default",
			podName:   "my-pod",
			validate: func(t *testing.T, secretName string) {
				if !strings.HasPrefix(secretName, "zen-lock-inject-default-my-pod-") {
					t.Errorf("Expected prefix 'zen-lock-inject-default-my-pod-', got %s", secretName)
				}
				if len(secretName) > 253 {
					t.Errorf("Secret name exceeds 253 chars: %d", len(secretName))
				}
			},
		},
		{
			name:      "short names",
			namespace: "ns",
			podName:   "pod",
			validate: func(t *testing.T, secretName string) {
				if !strings.HasPrefix(secretName, "zen-lock-inject-ns-pod-") {
					t.Errorf("Expected prefix 'zen-lock-inject-ns-pod-', got %s", secretName)
				}
			},
		},
		{
			name:      "long namespace",
			namespace: strings.Repeat("a", 100),
			podName:   strings.Repeat("b", 100),
			validate: func(t *testing.T, secretName string) {
				if len(secretName) > 253 {
					t.Errorf("Secret name exceeds 253 chars: %d", len(secretName))
				}
				// Should preserve hash suffix
				hash := sha256.Sum256([]byte("zen-lock-inject-" + strings.Repeat("a", 100) + "-" + strings.Repeat("b", 100)))
				hashStr := hex.EncodeToString(hash[:])[:16]
				if !strings.HasSuffix(secretName, hashStr) {
					t.Errorf("Expected hash suffix %s, got %s", hashStr, secretName)
				}
			},
		},
		{
			name:      "very long names",
			namespace: strings.Repeat("a", 200),
			podName:   strings.Repeat("b", 200),
			validate: func(t *testing.T, secretName string) {
				if len(secretName) > 253 {
					t.Errorf("Secret name exceeds 253 chars: %d", len(secretName))
				}
				// Should preserve hash suffix
				hash := sha256.Sum256([]byte("zen-lock-inject-" + strings.Repeat("a", 200) + "-" + strings.Repeat("b", 200)))
				hashStr := hex.EncodeToString(hash[:])[:16]
				if !strings.HasSuffix(secretName, hashStr) {
					t.Errorf("Expected hash suffix %s, got %s", hashStr, secretName)
				}
				// Name should be exactly 253 chars or less
				if len(secretName) > 253 {
					t.Errorf("Secret name should be <= 253 chars, got %d", len(secretName))
				}
			},
		},
		{
			name:      "stability check",
			namespace: "default",
			podName:   "my-pod",
			validate: func(t *testing.T, secretName string) {
				// Generate again and verify it's the same
				secretName2 := GenerateSecretName("default", "my-pod")
				if secretName != secretName2 {
					t.Errorf("Secret name should be stable, got %s and %s", secretName, secretName2)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secretName := GenerateSecretName(tt.namespace, tt.podName)
			if secretName == "" {
				t.Error("Secret name should not be empty")
			}
			tt.validate(t, secretName)
		})
	}
}

func TestGenerateSecretName_Uniqueness(t *testing.T) {
	names := make(map[string]bool)
	
	// Generate many secret names and verify uniqueness
	for i := 0; i < 100; i++ {
		namespace := "ns" + string(rune('a'+i%26))
		podName := "pod" + string(rune('0'+i%10))
		secretName := GenerateSecretName(namespace, podName)
		
		if names[secretName] {
			t.Errorf("Duplicate secret name generated: %s", secretName)
		}
		names[secretName] = true
	}
}

