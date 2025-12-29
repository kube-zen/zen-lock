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
	"os"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
)

func TestZenLockReconciler_SetupWithManager(t *testing.T) {
	// Save original value
	originalKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("ZEN_LOCK_PRIVATE_KEY", originalKey)
		} else {
			os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")
		}
	}()

	os.Setenv("ZEN_LOCK_PRIVATE_KEY", "AGE-SECRET-1EXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLE")

	scheme := runtime.NewScheme()
	if err := securityv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add securityv1alpha1 to scheme: %v", err)
	}

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	reconciler, err := NewZenLockReconciler(client, scheme)
	if err != nil {
		t.Fatalf("Failed to create reconciler: %v", err)
	}

	// Create a test manager (using fake manager if available, otherwise skip)
	// Note: This is a simplified test - in real scenarios, SetupWithManager is called by controller-runtime
	// We test that the function exists and can be called without panicking
	mgr, err := manager.New(client, manager.Options{
		Scheme: scheme,
	})
	if err != nil {
		t.Skipf("Failed to create manager (may require envtest): %v", err)
		return
	}

	err = reconciler.SetupWithManager(mgr)
	if err != nil {
		t.Errorf("SetupWithManager should not error: %v", err)
	}
}

