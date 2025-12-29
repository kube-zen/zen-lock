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
	"testing"
)

func TestValidateInjectAnnotation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid name",
			input:   "my-secret",
			wantErr: false,
		},
		{
			name:    "valid name with dots",
			input:   "my.secret.name",
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid uppercase",
			input:   "My-Secret",
			wantErr: true,
		},
		{
			name:    "invalid special chars",
			input:   "my_secret",
			wantErr: true,
		},
		{
			name:    "starts with dash",
			input:   "-my-secret",
			wantErr: true,
		},
		{
			name:    "ends with dash",
			input:   "my-secret-",
			wantErr: true,
		},
		{
			name:    "too long",
			input:   string(make([]byte, 254)),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateInjectAnnotation(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateInjectAnnotation() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateMountPath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid absolute path",
			input:   "/zen-lock/secrets",
			wantErr: false,
		},
		{
			name:    "valid nested path",
			input:   "/app/config/secrets",
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "relative path",
			input:   "secrets",
			wantErr: true,
		},
		{
			name:    "root path",
			input:   "/",
			wantErr: true,
		},
		{
			name:    "system directory",
			input:   "/etc/secrets",
			wantErr: true,
		},
		{
			name:    "system directory prefix",
			input:   "/bin/secrets",
			wantErr: true,
		},
		{
			name:    "directory traversal",
			input:   "/app/../etc/secrets",
			wantErr: true,
		},
		{
			name:    "too long",
			input:   "/" + string(make([]byte, 1025)),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMountPath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMountPath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		operation string
		check     func(string) bool
	}{
		{
			name:      "removes paths",
			err:       &testError{msg: "failed to read /etc/passwd"},
			operation: "read",
			check: func(s string) bool {
				return !contains(s, "/etc/passwd") && contains(s, "[path]")
			},
		},
		{
			name:      "removes long base64 strings",
			err:       &testError{msg: "secret: AGE-SECRET-1EXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLE"},
			operation: "decrypt",
			check: func(s string) bool {
				return !contains(s, "AGE-SECRET-1EXAMPLE") && contains(s, "[secret]")
			},
		},
		{
			name:      "removes IP addresses",
			err:       &testError{msg: "connection failed to 192.168.1.1"},
			operation: "connect",
			check: func(s string) bool {
				return !contains(s, "192.168.1.1") && contains(s, "[ip]")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitized := SanitizeError(tt.err, tt.operation)
			if sanitized == nil {
				t.Error("SanitizeError() returned nil")
				return
			}
			if !tt.check(sanitized.Error()) {
				t.Errorf("SanitizeError() = %v, did not pass check", sanitized.Error())
			}
		})
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsMiddle(s, substr))))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
