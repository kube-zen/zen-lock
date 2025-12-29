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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

func TestSecretReconciler_SetupWithManager(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	reconciler := NewSecretReconciler(client, scheme)

	// Create a test manager
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

