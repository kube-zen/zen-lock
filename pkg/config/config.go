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

package config

import "time"

// Default values for zen-lock configuration
const (
	// DefaultMountPath is the default mount path for injected secrets
	DefaultMountPath = "/zen-lock/secrets"

	// DefaultVolumeName is the default volume name for injected secrets
	DefaultVolumeName = "zen-secrets"

	// DefaultWebhookTimeout is the default timeout for webhook requests
	DefaultWebhookTimeout = 10 * time.Second

	// DefaultRetryMaxAttempts is the default maximum number of retry attempts
	DefaultRetryMaxAttempts = 3

	// DefaultRetryInitialDelay is the default initial delay between retries
	DefaultRetryInitialDelay = 100 * time.Millisecond

	// DefaultRetryMaxDelay is the default maximum delay between retries
	DefaultRetryMaxDelay = 2 * time.Second

	// DefaultWebhookRetryInitialDelay is the initial delay for webhook retries (shorter for faster response)
	DefaultWebhookRetryInitialDelay = 50 * time.Millisecond

	// DefaultWebhookRetryMaxDelay is the maximum delay for webhook retries
	DefaultWebhookRetryMaxDelay = 1 * time.Second

	// RequeueDelayPodNotFound is the delay when Pod is not found (allows time for Pod creation)
	RequeueDelayPodNotFound = 5 * time.Second

	// RequeueDelayPodNoUID is the delay when Pod exists but has no UID yet
	RequeueDelayPodNoUID = 2 * time.Second

	// DefaultAlgorithm is the default encryption algorithm
	DefaultAlgorithm = "age"

	// SupportedAlgorithm is the currently supported encryption algorithm
	SupportedAlgorithm = "age"
)

// Annotation keys
const (
	// AnnotationInject is the annotation key for specifying which ZenLock to inject
	AnnotationInject = "zen-lock/inject"

	// AnnotationMountPath is the annotation key for specifying a custom mount path
	AnnotationMountPath = "zen-lock/mount-path"
)
