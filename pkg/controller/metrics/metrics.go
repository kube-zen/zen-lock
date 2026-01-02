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

// Package metrics provides Prometheus metrics for zen-lock.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ZenLockReconcileTotal counts the total number of reconciliations.
	ZenLockReconcileTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zenlock_reconcile_total",
			Help: "Total number of ZenLock reconciliations",
		},
		[]string{"namespace", "name", "result"},
	)

	// ZenLockReconcileDuration measures the duration of reconciliations.
	ZenLockReconcileDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "zenlock_reconcile_duration_seconds",
			Help:    "Duration of ZenLock reconciliations in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
		[]string{"namespace", "name"},
	)

	// WebhookInjectionTotal counts the total number of webhook injections.
	WebhookInjectionTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zenlock_webhook_injection_total",
			Help: "Total number of webhook secret injections",
		},
		[]string{"namespace", "zenlock_name", "result"},
	)

	// WebhookInjectionDuration measures the duration of webhook injections.
	WebhookInjectionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "zenlock_webhook_injection_duration_seconds",
			Help:    "Duration of webhook secret injections in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
		[]string{"namespace", "zenlock_name"},
	)

	// DecryptionTotal counts the total number of decryption operations.
	DecryptionTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zenlock_decryption_total",
			Help: "Total number of decryption operations",
		},
		[]string{"namespace", "zenlock_name", "result"},
	)

	// DecryptionDuration measures the duration of decryption operations.
	DecryptionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "zenlock_decryption_duration_seconds",
			Help:    "Duration of decryption operations in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
		[]string{"namespace", "zenlock_name"},
	)

	// ZenLockCacheHits counts cache hits for ZenLock lookups.
	ZenLockCacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zenlock_cache_hits_total",
			Help: "Total number of ZenLock cache hits",
		},
		[]string{"namespace", "zenlock_name"},
	)

	// ZenLockCacheMisses counts cache misses for ZenLock lookups.
	ZenLockCacheMisses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zenlock_cache_misses_total",
			Help: "Total number of ZenLock cache misses",
		},
		[]string{"namespace", "zenlock_name"},
	)

	// WebhookValidationFailures counts validation failures in webhook.
	WebhookValidationFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zenlock_webhook_validation_failures_total",
			Help: "Total number of webhook validation failures",
		},
		[]string{"namespace", "reason"},
	)

	// AlgorithmUsageTotal counts algorithm usage by algorithm name.
	AlgorithmUsageTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zenlock_algorithm_usage_total",
			Help: "Total number of operations using each algorithm",
		},
		[]string{"algorithm", "operation"}, // operation: encrypt, decrypt
	)

	// AlgorithmErrorsTotal counts algorithm-related errors.
	AlgorithmErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zenlock_algorithm_errors_total",
			Help: "Total number of algorithm-related errors",
		},
		[]string{"algorithm", "reason"}, // reason: unsupported, invalid, decryption_failed
	)

	// CacheSizeGauge tracks the current cache size
	CacheSizeGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "zenlock_cache_size",
			Help: "Current number of entries in the ZenLock cache",
		},
	)

	// CacheHitRateGauge tracks the cache hit rate (hits / (hits + misses))
	CacheHitRateGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "zenlock_cache_hit_rate",
			Help: "Cache hit rate (hits / (hits + misses))",
		},
	)
)

// RecordReconcile records a reconciliation metric.
func RecordReconcile(namespace, name, result string, duration float64) {
	ZenLockReconcileTotal.WithLabelValues(namespace, name, result).Inc()
	ZenLockReconcileDuration.WithLabelValues(namespace, name).Observe(duration)
}

// RecordWebhookInjection records a webhook injection metric.
func RecordWebhookInjection(namespace, zenlockName, result string, duration float64) {
	WebhookInjectionTotal.WithLabelValues(namespace, zenlockName, result).Inc()
	WebhookInjectionDuration.WithLabelValues(namespace, zenlockName).Observe(duration)
}

// RecordDecryption records a decryption metric.
func RecordDecryption(namespace, zenlockName, result string, duration float64) {
	DecryptionTotal.WithLabelValues(namespace, zenlockName, result).Inc()
	DecryptionDuration.WithLabelValues(namespace, zenlockName).Observe(duration)
}

// RecordCacheHit records a cache hit.
func RecordCacheHit(namespace, zenlockName string) {
	ZenLockCacheHits.WithLabelValues(namespace, zenlockName).Inc()
}

// RecordCacheMiss records a cache miss.
func RecordCacheMiss(namespace, zenlockName string) {
	ZenLockCacheMisses.WithLabelValues(namespace, zenlockName).Inc()
}

// RecordValidationFailure records a validation failure.
func RecordValidationFailure(namespace, reason string) {
	WebhookValidationFailures.WithLabelValues(namespace, reason).Inc()
}

// RecordAlgorithmUsage records algorithm usage.
func RecordAlgorithmUsage(algorithm, operation string) {
	AlgorithmUsageTotal.WithLabelValues(algorithm, operation).Inc()
}

// RecordAlgorithmError records an algorithm-related error.
func RecordAlgorithmError(algorithm, reason string) {
	AlgorithmErrorsTotal.WithLabelValues(algorithm, reason).Inc()
}

// UpdateCacheMetrics updates cache size and hit rate metrics
func UpdateCacheMetrics(size int, hits, misses int64) {
	CacheSizeGauge.Set(float64(size))
	if hits+misses > 0 {
		hitRate := float64(hits) / float64(hits+misses)
		CacheHitRateGauge.Set(hitRate)
	} else {
		CacheHitRateGauge.Set(0)
	}
}
