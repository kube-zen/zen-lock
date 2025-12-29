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

package validation

import (
	"testing"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateZenLock(t *testing.T) {
	tests := []struct {
		name    string
		zenlock *securityv1alpha1.ZenLock
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil zenlock",
			zenlock: nil,
			wantErr: true,
			errMsg:  "zenlock is nil",
		},
		{
			name: "empty encryptedData",
			zenlock: &securityv1alpha1.ZenLock{
				Spec: securityv1alpha1.ZenLockSpec{
					EncryptedData: nil,
				},
			},
			wantErr: true,
			errMsg:  "encryptedData cannot be empty",
		},
		{
			name: "valid zenlock",
			zenlock: &securityv1alpha1.ZenLock{
				Spec: securityv1alpha1.ZenLockSpec{
					EncryptedData: map[string]string{
						"key1": "encrypted-value-1",
						"key2": "encrypted-value-2",
					},
					Algorithm: "age",
				},
			},
			wantErr: false,
		},
		{
			name: "unsupported algorithm",
			zenlock: &securityv1alpha1.ZenLock{
				Spec: securityv1alpha1.ZenLockSpec{
					EncryptedData: map[string]string{
						"key1": "value1",
					},
					Algorithm: "rsa",
				},
			},
			wantErr: true,
			errMsg:  "unsupported algorithm",
		},
		{
			name: "empty key in encryptedData",
			zenlock: &securityv1alpha1.ZenLock{
				Spec: securityv1alpha1.ZenLockSpec{
					EncryptedData: map[string]string{
						"": "value1",
					},
				},
			},
			wantErr: true,
			errMsg:  "encryptedData key cannot be empty",
		},
		{
			name: "empty value in encryptedData",
			zenlock: &securityv1alpha1.ZenLock{
				Spec: securityv1alpha1.ZenLockSpec{
					EncryptedData: map[string]string{
						"key1": "",
					},
				},
			},
			wantErr: true,
			errMsg:  "encryptedData value for key",
		},
		{
			name: "invalid allowedSubject",
			zenlock: &securityv1alpha1.ZenLock{
				Spec: securityv1alpha1.ZenLockSpec{
					EncryptedData: map[string]string{
						"key1": "value1",
					},
					AllowedSubjects: []securityv1alpha1.SubjectReference{
						{
							Kind: "ServiceAccount",
							// Missing name
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateZenLock(tt.zenlock)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateZenLock() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil || err.Error() == "" {
					t.Errorf("ValidateZenLock() expected error message containing %q, got %v", tt.errMsg, err)
				}
			}
		})
	}
}

func TestValidateSubjectReference(t *testing.T) {
	tests := []struct {
		name    string
		subject *securityv1alpha1.SubjectReference
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil subject",
			subject: nil,
			wantErr: true,
			errMsg:  "subject is nil",
		},
		{
			name: "missing kind",
			subject: &securityv1alpha1.SubjectReference{
				Name: "test",
			},
			wantErr: true,
			errMsg:  "kind is required",
		},
		{
			name: "missing name",
			subject: &securityv1alpha1.SubjectReference{
				Kind: "ServiceAccount",
			},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "invalid kind",
			subject: &securityv1alpha1.SubjectReference{
				Kind: "InvalidKind",
				Name: "test",
			},
			wantErr: true,
			errMsg:  "invalid kind",
		},
		{
			name: "ServiceAccount without namespace",
			subject: &securityv1alpha1.SubjectReference{
				Kind: "ServiceAccount",
				Name: "test",
			},
			wantErr: true,
			errMsg:  "namespace is required for ServiceAccount",
		},
		{
			name: "valid ServiceAccount",
			subject: &securityv1alpha1.SubjectReference{
				Kind:      "ServiceAccount",
				Name:      "test",
				Namespace: "default",
			},
			wantErr: false,
		},
		{
			name: "valid User",
			subject: &securityv1alpha1.SubjectReference{
				Kind: "User",
				Name: "test-user",
			},
			wantErr: false,
		},
		{
			name: "valid Group",
			subject: &securityv1alpha1.SubjectReference{
				Kind: "Group",
				Name: "test-group",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSubjectReference(tt.subject)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSubjectReference() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil || err.Error() == "" {
					t.Errorf("ValidateSubjectReference() expected error message containing %q, got %v", tt.errMsg, err)
				}
			}
		})
	}
}

func TestValidateZenLock_WithAllowedSubjects(t *testing.T) {
	zenlock := &securityv1alpha1.ZenLock{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Spec: securityv1alpha1.ZenLockSpec{
			EncryptedData: map[string]string{
				"key1": "encrypted-value-1",
			},
			AllowedSubjects: []securityv1alpha1.SubjectReference{
				{
					Kind:      "ServiceAccount",
					Name:      "backend-app",
					Namespace: "default",
				},
			},
		},
	}

	err := ValidateZenLock(zenlock)
	if err != nil {
		t.Errorf("ValidateZenLock() with valid allowedSubjects should not error, got: %v", err)
	}
}
