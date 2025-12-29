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

func TestRecordReconcile(t *testing.T) {
	RecordReconcile("default", "test-zenlock", "success", 0.1)
	RecordReconcile("default", "test-zenlock", "error", 0.2)
	
	// Verify metrics are recorded (check that no panic occurs)
	// In a real scenario, we'd use a test prometheus registry
}

func TestRecordWebhookInjection(t *testing.T) {
	RecordWebhookInjection("default", "test-zenlock", "success", 0.05)
	RecordWebhookInjection("default", "test-zenlock", "error", 0.1)
	RecordWebhookInjection("default", "test-zenlock", "denied", 0.02)
}

func TestRecordDecryption(t *testing.T) {
	RecordDecryption("default", "test-zenlock", "success", 0.01)
	RecordDecryption("default", "test-zenlock", "error", 0.02)
}

func TestRecordCacheHit(t *testing.T) {
	RecordCacheHit("default", "test-zenlock")
}

func TestRecordCacheMiss(t *testing.T) {
	RecordCacheMiss("default", "test-zenlock")
}

func TestRecordValidationFailure(t *testing.T) {
	RecordValidationFailure("default", "invalid_inject_annotation")
	RecordValidationFailure("default", "invalid_mount_path")
}

func TestRecordAlgorithmUsage(t *testing.T) {
	RecordAlgorithmUsage("age", "encrypt")
	RecordAlgorithmUsage("age", "decrypt")
}

func TestRecordAlgorithmError(t *testing.T) {
	RecordAlgorithmError("age", "unsupported")
	RecordAlgorithmError("age", "invalid")
	RecordAlgorithmError("age", "decryption_failed")
}

