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

package common

// Label keys for zen-lock Secrets
const (
	// LabelPodName identifies the Pod name associated with a zen-lock Secret
	LabelPodName = "zen-lock.security.kube-zen.io/pod-name"

	// LabelPodNamespace identifies the Pod namespace associated with a zen-lock Secret
	LabelPodNamespace = "zen-lock.security.kube-zen.io/pod-namespace"

	// LabelZenLockName identifies the ZenLock CRD name associated with a zen-lock Secret
	LabelZenLockName = "zen-lock.security.kube-zen.io/zenlock-name"
)
