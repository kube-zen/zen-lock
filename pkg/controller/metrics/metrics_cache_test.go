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

package metrics

import (
	"testing"
)

func TestUpdateCacheMetrics(t *testing.T) {
	// Test with hits and misses
	UpdateCacheMetrics(10, 8, 2)

	// Verify no panic occurred
	// In a real scenario, we'd check the actual metric values using a test registry

	// Test with zero hits and misses
	UpdateCacheMetrics(5, 0, 0)

	// Test with only hits
	UpdateCacheMetrics(3, 10, 0)

	// Test with only misses
	UpdateCacheMetrics(3, 0, 10)

	// Test with large numbers
	UpdateCacheMetrics(100, 75, 25)
}
