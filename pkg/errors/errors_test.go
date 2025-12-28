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

package errors

import (
	"errors"
	"testing"
)

func TestZenLockError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *ZenLockError
		wantErr string
	}{
		{
			name: "error with message only",
			err: &ZenLockError{
				Type:    "test_error",
				Message: "test message",
			},
			wantErr: "test message",
		},
		{
			name: "error with underlying error",
			err: &ZenLockError{
				Type:    "test_error",
				Message: "test message",
				Err:     errors.New("underlying error"),
			},
			wantErr: "test message: underlying error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantErr {
				t.Errorf("ZenLockError.Error() = %v, want %v", got, tt.wantErr)
			}
		})
	}
}

func TestZenLockError_Unwrap(t *testing.T) {
	underlying := errors.New("underlying error")
	err := &ZenLockError{
		Type:    "test_error",
		Message: "test message",
		Err:     underlying,
	}

	if unwrapped := err.Unwrap(); unwrapped != underlying {
		t.Errorf("ZenLockError.Unwrap() = %v, want %v", unwrapped, underlying)
	}

	errNoWrap := &ZenLockError{
		Type:    "test_error",
		Message: "test message",
	}
	if unwrapped := errNoWrap.Unwrap(); unwrapped != nil {
		t.Errorf("ZenLockError.Unwrap() = %v, want nil", unwrapped)
	}
}

func TestWithZenLock(t *testing.T) {
	originalErr := errors.New("original error")
	err := WithZenLock(originalErr, "test-namespace", "test-name")

	if err.ZenLockNamespace != "test-namespace" {
		t.Errorf("ZenLockNamespace = %v, want test-namespace", err.ZenLockNamespace)
	}
	if err.ZenLockName != "test-name" {
		t.Errorf("ZenLockName = %v, want test-name", err.ZenLockName)
	}
	if err.Err != originalErr {
		t.Errorf("Err = %v, want %v", err.Err, originalErr)
	}
}

func TestWithPod(t *testing.T) {
	originalErr := errors.New("original error")
	err := WithPod(originalErr, "test-namespace", "test-pod")

	if err.PodNamespace != "test-namespace" {
		t.Errorf("PodNamespace = %v, want test-namespace", err.PodNamespace)
	}
	if err.PodName != "test-pod" {
		t.Errorf("PodName = %v, want test-pod", err.PodName)
	}
	if err.Err != originalErr {
		t.Errorf("Err = %v, want %v", err.Err, originalErr)
	}
}

func TestNew(t *testing.T) {
	err := New("test_type", "test message")

	if err.Type != "test_type" {
		t.Errorf("Type = %v, want test_type", err.Type)
	}
	if err.Message != "test message" {
		t.Errorf("Message = %v, want test message", err.Message)
	}
	if err.Err != nil {
		t.Errorf("Err = %v, want nil", err.Err)
	}
}

func TestWrap(t *testing.T) {
	originalErr := errors.New("original error")
	err := Wrap(originalErr, "test_type", "test message")

	if err.Type != "test_type" {
		t.Errorf("Type = %v, want test_type", err.Type)
	}
	if err.Message != "test message" {
		t.Errorf("Message = %v, want test message", err.Message)
	}
	if err.Err != originalErr {
		t.Errorf("Err = %v, want %v", err.Err, originalErr)
	}
}

func TestWrapf(t *testing.T) {
	originalErr := errors.New("original error")
	err := Wrapf(originalErr, "test_type", "test %s", "message")

	if err.Type != "test_type" {
		t.Errorf("Type = %v, want test_type", err.Type)
	}
	if err.Message != "test message" {
		t.Errorf("Message = %v, want test message", err.Message)
	}
	if err.Err != originalErr {
		t.Errorf("Err = %v, want %v", err.Err, originalErr)
	}
}

func TestWithZenLock_PreservesExistingContext(t *testing.T) {
	originalErr := errors.New("original error")
	err1 := WithPod(originalErr, "pod-namespace", "pod-name")
	err2 := WithZenLock(err1, "test-namespace", "test-name")

	if err2.PodNamespace != "pod-namespace" {
		t.Errorf("PodNamespace should be preserved: got %v, want pod-namespace", err2.PodNamespace)
	}
	if err2.PodName != "pod-name" {
		t.Errorf("PodName should be preserved: got %v, want pod-name", err2.PodName)
	}
	if err2.ZenLockNamespace != "test-namespace" {
		t.Errorf("ZenLockNamespace = %v, want test-namespace", err2.ZenLockNamespace)
	}
	if err2.ZenLockName != "test-name" {
		t.Errorf("ZenLockName = %v, want test-name", err2.ZenLockName)
	}
}
