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

// Note: TestWithPod_PreservesExistingZenLockError and TestWithPod_WithNilError
// are already in errors_test_enhanced.go. These tests cover additional edge cases.

func TestWithPod_RegularError(t *testing.T) {
	// Test WithPod with a regular (non-ZenLockError) error
	regularErr := errors.New("regular error")
	podErr := WithPod(regularErr, "pod-ns", "pod-name")

	if podErr.PodNamespace != "pod-ns" {
		t.Errorf("Expected PodNamespace to be 'pod-ns', got %v", podErr.PodNamespace)
	}
	if podErr.PodName != "pod-name" {
		t.Errorf("Expected PodName to be 'pod-name', got %v", podErr.PodName)
	}
	if podErr.Message != "regular error" {
		t.Errorf("Expected Message to be 'regular error', got %v", podErr.Message)
	}
	if podErr.Err != regularErr {
		t.Errorf("Expected Err to be the original error")
	}
}

func TestWithPod_WithExistingZenLockContext(t *testing.T) {
	// Test WithPod when error already has ZenLock context
	originalErr := errors.New("original error")
	zenlockErr := WithZenLock(originalErr, "zenlock-ns", "zenlock-name")
	podErr := WithPod(zenlockErr, "pod-ns", "pod-name")

	// Should preserve ZenLock context
	if podErr.ZenLockNamespace != "zenlock-ns" {
		t.Errorf("Expected ZenLockNamespace to be preserved: got %v, want zenlock-ns", podErr.ZenLockNamespace)
	}
	if podErr.ZenLockName != "zenlock-name" {
		t.Errorf("Expected ZenLockName to be preserved: got %v, want zenlock-name", podErr.ZenLockName)
	}
	// Should add Pod context
	if podErr.PodNamespace != "pod-ns" {
		t.Errorf("Expected PodNamespace = %v, want pod-ns", podErr.PodNamespace)
	}
	if podErr.PodName != "pod-name" {
		t.Errorf("Expected PodName = %v, want pod-name", podErr.PodName)
	}
}

