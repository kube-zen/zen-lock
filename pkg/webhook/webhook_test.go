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
	"os"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	corev1 "k8s.io/api/core/v1"
	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
)

func TestSetupWebhookWithManager(t *testing.T) {
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
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(securityv1alpha1.AddToScheme(scheme))

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Create a test manager
	mgr, err := manager.New(client, manager.Options{
		Scheme: scheme,
	})
	if err != nil {
		t.Skipf("Failed to create manager (may require envtest): %v", err)
		return
	}

	err = SetupWebhookWithManager(mgr)
	if err != nil {
		t.Errorf("SetupWebhookWithManager should not error: %v", err)
	}
}

func TestSetupWebhookWithManager_NoPrivateKey(t *testing.T) {
	// Save original value
	originalKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("ZEN_LOCK_PRIVATE_KEY", originalKey)
		} else {
			os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")
		}
	}()

	os.Unsetenv("ZEN_LOCK_PRIVATE_KEY")

	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(securityv1alpha1.AddToScheme(scheme))

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	mgr, err := manager.New(client, manager.Options{
		Scheme: scheme,
	})
	if err != nil {
		t.Skipf("Failed to create manager (may require envtest): %v", err)
		return
	}

	err = SetupWebhookWithManager(mgr)
	if err == nil {
		t.Error("SetupWebhookWithManager should error when ZEN_LOCK_PRIVATE_KEY is not set")
	}
}

