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

func TestWithPod_PreservesExistingZenLockError(t *testing.T) {
	originalErr := errors.New("original error")
	err1 := WithZenLock(originalErr, "zenlock-namespace", "zenlock-name")
	err2 := WithPod(err1, "pod-namespace", "pod-name")

	// Should preserve ZenLock context
	if err2.GetContext("zenlock_namespace") != "zenlock-namespace" {
		t.Errorf("zenlock_namespace should be preserved: got %v, want zenlock-namespace", err2.GetContext("zenlock_namespace"))
	}
	if err2.GetContext("zenlock_name") != "zenlock-name" {
		t.Errorf("zenlock_name should be preserved: got %v, want zenlock-name", err2.GetContext("zenlock_name"))
	}
	// Should add Pod context
	if err2.GetContext("pod_namespace") != "pod-namespace" {
		t.Errorf("pod_namespace = %v, want pod-namespace", err2.GetContext("pod_namespace"))
	}
	if err2.GetContext("pod_name") != "pod-name" {
		t.Errorf("pod_name = %v, want pod-name", err2.GetContext("pod_name"))
	}
}

func TestWithPod_WithNilError(t *testing.T) {
	// Test that WithPod handles nil gracefully (if it does)
	// This tests the error.As path when err is nil
	var nilErr error
	err := WithPod(nilErr, "namespace", "name")

	// Should still create an error with Pod context
	if err.GetContext("pod_namespace") != "namespace" {
		t.Errorf("pod_namespace = %v, want namespace", err.GetContext("pod_namespace"))
	}
	if err.GetContext("pod_name") != "name" {
		t.Errorf("pod_name = %v, want name", err.GetContext("pod_name"))
	}
}
