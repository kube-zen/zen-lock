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

