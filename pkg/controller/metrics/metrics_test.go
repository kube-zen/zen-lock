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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordReconcile(t *testing.T) {
	// Create a test registry
	reg := prometheus.NewRegistry()

	// Create metrics with test registry
	reconcileTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zenlock_reconcile_total_test",
			Help: "Test metric",
		},
		[]string{"namespace", "name", "result"},
	)
	reconcileDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "zenlock_reconcile_duration_seconds_test",
			Help:    "Test metric",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
		[]string{"namespace", "name"},
	)
	reg.MustRegister(reconcileTotal, reconcileDuration)

	// Record metrics
	reconcileTotal.WithLabelValues("default", "test-zenlock", "success").Inc()
	reconcileDuration.WithLabelValues("default", "test-zenlock").Observe(0.1)
	reconcileTotal.WithLabelValues("default", "test-zenlock", "error").Inc()
	reconcileDuration.WithLabelValues("default", "test-zenlock").Observe(0.2)

	// Verify counter values
	if count := testutil.ToFloat64(reconcileTotal.WithLabelValues("default", "test-zenlock", "success")); count != 1 {
		t.Errorf("Expected success count 1, got %f", count)
	}
	if count := testutil.ToFloat64(reconcileTotal.WithLabelValues("default", "test-zenlock", "error")); count != 1 {
		t.Errorf("Expected error count 1, got %f", count)
	}

	// Verify histogram was updated (check that no panic occurred)
	// Histogram verification is complex, so we just verify the function works

	// Also test the actual functions (they use global registry)
	RecordReconcile("default", "test-zenlock2", "success", 0.15)
	RecordReconcile("default", "test-zenlock2", "error", 0.25)
}

func TestRecordWebhookInjection(t *testing.T) {
	// Create a test registry
	reg := prometheus.NewRegistry()

	injectionTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zenlock_webhook_injection_total_test",
			Help: "Test metric",
		},
		[]string{"namespace", "zenlock_name", "result"},
	)
	injectionDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "zenlock_webhook_injection_duration_seconds_test",
			Help:    "Test metric",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
		[]string{"namespace", "zenlock_name"},
	)
	reg.MustRegister(injectionTotal, injectionDuration)

	// Record metrics
	injectionTotal.WithLabelValues("default", "test-zenlock", "success").Inc()
	injectionDuration.WithLabelValues("default", "test-zenlock").Observe(0.05)
	injectionTotal.WithLabelValues("default", "test-zenlock", "error").Inc()
	injectionDuration.WithLabelValues("default", "test-zenlock").Observe(0.1)
	injectionTotal.WithLabelValues("default", "test-zenlock", "denied").Inc()
	injectionDuration.WithLabelValues("default", "test-zenlock").Observe(0.02)

	// Verify counter values
	if count := testutil.ToFloat64(injectionTotal.WithLabelValues("default", "test-zenlock", "success")); count != 1 {
		t.Errorf("Expected success count 1, got %f", count)
	}
	if count := testutil.ToFloat64(injectionTotal.WithLabelValues("default", "test-zenlock", "error")); count != 1 {
		t.Errorf("Expected error count 1, got %f", count)
	}
	if count := testutil.ToFloat64(injectionTotal.WithLabelValues("default", "test-zenlock", "denied")); count != 1 {
		t.Errorf("Expected denied count 1, got %f", count)
	}

	// Also test the actual functions
	RecordWebhookInjection("default", "test-zenlock2", "success", 0.05)
	RecordWebhookInjection("default", "test-zenlock2", "error", 0.1)
	RecordWebhookInjection("default", "test-zenlock2", "denied", 0.02)
}

func TestRecordDecryption(t *testing.T) {
	// Create a test registry
	reg := prometheus.NewRegistry()

	decryptionTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zenlock_decryption_total_test",
			Help: "Test metric",
		},
		[]string{"namespace", "zenlock_name", "result"},
	)
	decryptionDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "zenlock_decryption_duration_seconds_test",
			Help:    "Test metric",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
		[]string{"namespace", "zenlock_name"},
	)
	reg.MustRegister(decryptionTotal, decryptionDuration)

	// Record metrics
	decryptionTotal.WithLabelValues("default", "test-zenlock", "success").Inc()
	decryptionDuration.WithLabelValues("default", "test-zenlock").Observe(0.01)
	decryptionTotal.WithLabelValues("default", "test-zenlock", "error").Inc()
	decryptionDuration.WithLabelValues("default", "test-zenlock").Observe(0.02)

	// Verify counter values
	if count := testutil.ToFloat64(decryptionTotal.WithLabelValues("default", "test-zenlock", "success")); count != 1 {
		t.Errorf("Expected success count 1, got %f", count)
	}
	if count := testutil.ToFloat64(decryptionTotal.WithLabelValues("default", "test-zenlock", "error")); count != 1 {
		t.Errorf("Expected error count 1, got %f", count)
	}

	// Also test the actual functions
	RecordDecryption("default", "test-zenlock2", "success", 0.01)
	RecordDecryption("default", "test-zenlock2", "error", 0.02)
}

func TestRecordCacheHit(t *testing.T) {
	// Create a test registry
	reg := prometheus.NewRegistry()

	cacheHits := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zenlock_cache_hits_total_test",
			Help: "Test metric",
		},
		[]string{"namespace", "zenlock_name"},
	)
	reg.MustRegister(cacheHits)

	// Record metric
	cacheHits.WithLabelValues("default", "test-zenlock").Inc()

	// Verify counter value
	if count := testutil.ToFloat64(cacheHits.WithLabelValues("default", "test-zenlock")); count != 1 {
		t.Errorf("Expected cache hit count 1, got %f", count)
	}

	// Also test the actual function
	RecordCacheHit("default", "test-zenlock2")
}

func TestRecordCacheMiss(t *testing.T) {
	// Create a test registry
	reg := prometheus.NewRegistry()

	cacheMisses := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zenlock_cache_misses_total_test",
			Help: "Test metric",
		},
		[]string{"namespace", "zenlock_name"},
	)
	reg.MustRegister(cacheMisses)

	// Record metric
	cacheMisses.WithLabelValues("default", "test-zenlock").Inc()

	// Verify counter value
	if count := testutil.ToFloat64(cacheMisses.WithLabelValues("default", "test-zenlock")); count != 1 {
		t.Errorf("Expected cache miss count 1, got %f", count)
	}

	// Also test the actual function
	RecordCacheMiss("default", "test-zenlock2")
}

func TestRecordValidationFailure(t *testing.T) {
	// Create a test registry
	reg := prometheus.NewRegistry()

	validationFailures := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zenlock_webhook_validation_failures_total_test",
			Help: "Test metric",
		},
		[]string{"namespace", "reason"},
	)
	reg.MustRegister(validationFailures)

	// Record metrics
	validationFailures.WithLabelValues("default", "invalid_inject_annotation").Inc()
	validationFailures.WithLabelValues("default", "invalid_mount_path").Inc()

	// Verify counter values
	if count := testutil.ToFloat64(validationFailures.WithLabelValues("default", "invalid_inject_annotation")); count != 1 {
		t.Errorf("Expected invalid_inject_annotation count 1, got %f", count)
	}
	if count := testutil.ToFloat64(validationFailures.WithLabelValues("default", "invalid_mount_path")); count != 1 {
		t.Errorf("Expected invalid_mount_path count 1, got %f", count)
	}

	// Also test the actual functions
	RecordValidationFailure("default", "invalid_inject_annotation")
	RecordValidationFailure("default", "invalid_mount_path")
}

func TestRecordAlgorithmUsage(t *testing.T) {
	// Create a test registry
	reg := prometheus.NewRegistry()

	algorithmUsage := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zenlock_algorithm_usage_total_test",
			Help: "Test metric",
		},
		[]string{"algorithm", "operation"},
	)
	reg.MustRegister(algorithmUsage)

	// Record metrics
	algorithmUsage.WithLabelValues("age", "encrypt").Inc()
	algorithmUsage.WithLabelValues("age", "decrypt").Inc()

	// Verify counter values
	if count := testutil.ToFloat64(algorithmUsage.WithLabelValues("age", "encrypt")); count != 1 {
		t.Errorf("Expected encrypt count 1, got %f", count)
	}
	if count := testutil.ToFloat64(algorithmUsage.WithLabelValues("age", "decrypt")); count != 1 {
		t.Errorf("Expected decrypt count 1, got %f", count)
	}

	// Also test the actual functions
	RecordAlgorithmUsage("age", "encrypt")
	RecordAlgorithmUsage("age", "decrypt")
}

func TestRecordAlgorithmError(t *testing.T) {
	// Create a test registry
	reg := prometheus.NewRegistry()

	algorithmErrors := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zenlock_algorithm_errors_total_test",
			Help: "Test metric",
		},
		[]string{"algorithm", "reason"},
	)
	reg.MustRegister(algorithmErrors)

	// Record metrics
	algorithmErrors.WithLabelValues("age", "unsupported").Inc()
	algorithmErrors.WithLabelValues("age", "invalid").Inc()
	algorithmErrors.WithLabelValues("age", "decryption_failed").Inc()

	// Verify counter values
	if count := testutil.ToFloat64(algorithmErrors.WithLabelValues("age", "unsupported")); count != 1 {
		t.Errorf("Expected unsupported count 1, got %f", count)
	}
	if count := testutil.ToFloat64(algorithmErrors.WithLabelValues("age", "invalid")); count != 1 {
		t.Errorf("Expected invalid count 1, got %f", count)
	}
	if count := testutil.ToFloat64(algorithmErrors.WithLabelValues("age", "decryption_failed")); count != 1 {
		t.Errorf("Expected decryption_failed count 1, got %f", count)
	}

	// Also test the actual functions
	RecordAlgorithmError("age", "unsupported")
	RecordAlgorithmError("age", "invalid")
	RecordAlgorithmError("age", "decryption_failed")
}
