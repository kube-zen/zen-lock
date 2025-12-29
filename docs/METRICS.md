# Metrics Documentation

This document describes all Prometheus metrics exposed by the zen-lock controller and webhook.

## Metrics Endpoint

The zen-lock controller exposes metrics on the `/metrics` endpoint, defaulting to port `8080`.

## Available Metrics

### `zenlock_reconcile_total`
**Type**: Counter  
**Description**: Total number of ZenLock reconciliations  
**Labels**:
- `namespace`: Namespace of the ZenLock
- `name`: Name of the ZenLock
- `result`: Result of reconciliation (`success`, `error`)

**Example**:
```
zenlock_reconcile_total{namespace="default",name="app-secrets",result="success"} 150
zenlock_reconcile_total{namespace="default",name="app-secrets",result="error"} 2
```

---

### `zenlock_reconcile_duration_seconds`
**Type**: Histogram  
**Description**: Duration of ZenLock reconciliations in seconds  
**Labels**:
- `namespace`: Namespace of the ZenLock
- `name`: Name of the ZenLock

**Buckets**: Exponential buckets from 0.001s to ~1s (0.001, 0.002, 0.004, 0.008, 0.016, 0.032, 0.064, 0.128, 0.256, 0.512, 1.024)

**Example**:
```
zenlock_reconcile_duration_seconds_bucket{namespace="default",name="app-secrets",le="0.1"} 145
zenlock_reconcile_duration_seconds_sum{namespace="default",name="app-secrets"} 12.5
zenlock_reconcile_duration_seconds_count{namespace="default",name="app-secrets"} 150
```

---

### `zenlock_webhook_injection_total`
**Type**: Counter  
**Description**: Total number of webhook secret injections  
**Labels**:
- `namespace`: Namespace of the Pod
- `zenlock_name`: Name of the ZenLock being injected
- `result`: Result of injection (`success`, `error`, `denied`)

**Example**:
```
zenlock_webhook_injection_total{namespace="default",zenlock_name="app-secrets",result="success"} 500
zenlock_webhook_injection_total{namespace="default",zenlock_name="app-secrets",result="denied"} 5
zenlock_webhook_injection_total{namespace="default",zenlock_name="app-secrets",result="error"} 2
```

---

### `zenlock_webhook_injection_duration_seconds`
**Type**: Histogram  
**Description**: Duration of webhook secret injections in seconds  
**Labels**:
- `namespace`: Namespace of the Pod
- `zenlock_name`: Name of the ZenLock being injected

**Buckets**: Exponential buckets from 0.001s to ~1s

**Example**:
```
zenlock_webhook_injection_duration_seconds_bucket{namespace="default",zenlock_name="app-secrets",le="0.1"} 495
zenlock_webhook_injection_duration_seconds_sum{namespace="default",zenlock_name="app-secrets"} 25.3
zenlock_webhook_injection_duration_seconds_count{namespace="default",zenlock_name="app-secrets"} 500
```

---

### `zenlock_decryption_total`
**Type**: Counter  
**Description**: Total number of decryption operations  
**Labels**:
- `namespace`: Namespace of the ZenLock
- `zenlock_name`: Name of the ZenLock
- `result`: Result of decryption (`success`, `error`)

**Example**:
```
zenlock_decryption_total{namespace="default",zenlock_name="app-secrets",result="success"} 650
zenlock_decryption_total{namespace="default",zenlock_name="app-secrets",result="error"} 1
```

---

### `zenlock_decryption_duration_seconds`
**Type**: Histogram  
**Description**: Duration of decryption operations in seconds  
**Labels**:
- `namespace`: Namespace of the ZenLock
- `zenlock_name`: Name of the ZenLock

**Buckets**: Exponential buckets from 0.001s to ~1s

**Example**:
```
zenlock_decryption_duration_seconds_bucket{namespace="default",zenlock_name="app-secrets",le="0.01"} 640
zenlock_decryption_duration_seconds_sum{namespace="default",zenlock_name="app-secrets"} 3.2
zenlock_decryption_duration_seconds_count{namespace="default",zenlock_name="app-secrets"} 650
```

---

### `zenlock_cache_hits_total`
**Type**: Counter  
**Description**: Total number of ZenLock cache hits (reduces API server load)  
**Labels**:
- `namespace`: Namespace of the ZenLock
- `zenlock_name`: Name of the ZenLock

**Example**:
```
zenlock_cache_hits_total{namespace="default",zenlock_name="app-secrets"} 450
```

---

### `zenlock_cache_misses_total`
**Type**: Counter  
**Description**: Total number of ZenLock cache misses (API server lookups)  
**Labels**:
- `namespace`: Namespace of the ZenLock
- `zenlock_name`: Name of the ZenLock

**Example**:
```
zenlock_cache_misses_total{namespace="default",zenlock_name="app-secrets"} 50
```

**Cache Hit Rate**:
```promql
sum(rate(zenlock_cache_hits_total[5m])) 
/ 
(sum(rate(zenlock_cache_hits_total[5m])) + sum(rate(zenlock_cache_misses_total[5m]))) * 100
```

---

### `zenlock_webhook_validation_failures_total`
**Type**: Counter  
**Description**: Total number of webhook validation failures  
**Labels**:
- `namespace`: Namespace of the Pod
- `reason`: Reason for validation failure (`invalid_inject_annotation`, `invalid_mount_path`, etc.)

**Example**:
```
zenlock_webhook_validation_failures_total{namespace="default",reason="invalid_inject_annotation"} 2
zenlock_webhook_validation_failures_total{namespace="default",reason="invalid_mount_path"} 1
```

---

### `zenlock_algorithm_usage_total`
**Type**: Counter  
**Description**: Total number of operations using each algorithm  
**Labels**:
- `algorithm`: Algorithm name (e.g., `age`, `aes256-gcm`)
- `operation`: Operation type (`encrypt`, `decrypt`)

**Example**:
```
zenlock_algorithm_usage_total{algorithm="age",operation="encrypt"} 1000
zenlock_algorithm_usage_total{algorithm="age",operation="decrypt"} 1500
```

**Use Cases**:
- Track which algorithms are being used in your cluster
- Monitor algorithm adoption over time
- Identify ZenLocks using deprecated algorithms

---

### `zenlock_algorithm_errors_total`
**Type**: Counter  
**Description**: Total number of algorithm-related errors  
**Labels**:
- `algorithm`: Algorithm name (or `unknown` if algorithm cannot be determined)
- `reason`: Error reason (`unsupported`, `invalid`, `decryption_failed`)

**Example**:
```
zenlock_algorithm_errors_total{algorithm="rsa",reason="unsupported"} 5
zenlock_algorithm_errors_total{algorithm="age",reason="decryption_failed"} 2
```

**Use Cases**:
- Alert on unsupported algorithm usage
- Track algorithm validation failures
- Monitor decryption failures by algorithm

---

## Prometheus Queries

### Reconciliation Success Rate
```promql
sum(rate(zenlock_reconcile_total{result="success"}[5m])) 
/ 
sum(rate(zenlock_reconcile_total[5m])) * 100
```

### Webhook Injection Success Rate
```promql
sum(rate(zenlock_webhook_injection_total{result="success"}[5m])) 
/ 
sum(rate(zenlock_webhook_injection_total[5m])) * 100
```

### Decryption Success Rate
```promql
sum(rate(zenlock_decryption_total{result="success"}[5m])) 
/ 
sum(rate(zenlock_decryption_total[5m])) * 100
```

### P95 Reconciliation Duration
```promql
histogram_quantile(0.95, 
  sum(rate(zenlock_reconcile_duration_seconds_bucket[5m])) by (le, namespace, name)
)
```

### P95 Webhook Injection Duration
```promql
histogram_quantile(0.95, 
  sum(rate(zenlock_webhook_injection_duration_seconds_bucket[5m])) by (le, namespace, zenlock_name)
)
```

### P95 Decryption Duration
```promql
histogram_quantile(0.95, 
  sum(rate(zenlock_decryption_duration_seconds_bucket[5m])) by (le, namespace, zenlock_name)
)
```

### Error Rate by Type
```promql
sum(rate(zenlock_reconcile_total{result="error"}[5m])) by (namespace, name)
```

### Webhook Denial Rate (AllowedSubjects)
```promql
sum(rate(zenlock_webhook_injection_total{result="denied"}[5m])) by (namespace, zenlock_name)
```

### Top ZenLocks by Injection Count
```promql
topk(10, sum(rate(zenlock_webhook_injection_total{result="success"}[5m])) by (namespace, zenlock_name))
```

---

## Grafana Dashboard

A Grafana dashboard is available at `deploy/grafana/dashboard.json` with pre-configured panels for:
- Reconciliation metrics
- Webhook injection metrics
- Decryption metrics
- Error rates
- Duration histograms
- Top ZenLocks by usage
- Algorithm usage distribution
- Algorithm usage over time
- Algorithm errors

See [Grafana Dashboard README](../deploy/grafana/README.md) for installation instructions.

---

## Alerting Rules

Prometheus alerting rules are available at `deploy/prometheus/prometheus-rules.yaml`:

- **ZenLockControllerDown**: Alerts when controller is down
- **ZenLockHighReconciliationErrorRate**: Alerts on high reconciliation error rates (>5 errors/sec)
- **ZenLockWebhookInjectionFailures**: Alerts on webhook injection failures (>2 failures/sec)
- **ZenLockWebhookInjectionDenials**: Alerts on injection denials (AllowedSubjects violations)
- **ZenLockDecryptionFailures**: Alerts on decryption failures (>3 failures/sec)
- **ZenLockUnsupportedAlgorithm**: Alerts on unsupported algorithm usage
- **ZenLockAlgorithmValidationFailures**: Alerts on algorithm validation failures (>2 failures/sec)
- **ZenLockSlowReconciliation**: Alerts on slow reconciliations (P95 >5s)
- **ZenLockSlowWebhookInjection**: Alerts on slow webhook injections (P95 >2s)
- **ZenLockSlowDecryption**: Alerts on slow decryption operations (P95 >1s)

---

## Metric Collection Flow

1. **Controller Reconciliation**:
   - `Reconcile()` is called for each ZenLock
   - Metrics recorded: `zenlock_reconcile_total`, `zenlock_reconcile_duration_seconds`
   - Decryption metrics recorded: `zenlock_decryption_total`, `zenlock_decryption_duration_seconds`

2. **Webhook Injection**:
   - `Handle()` processes Pod admission requests
   - Metrics recorded: `zenlock_webhook_injection_total`, `zenlock_webhook_injection_duration_seconds`
   - Decryption metrics recorded during secret decryption

3. **Metrics Exposure**:
   - Controller-runtime automatically exposes metrics via `/metrics` endpoint
   - Prometheus scrapes metrics from port `8080`

---

## See Also

- [Architecture](ARCHITECTURE.md) - How metrics are collected
- [User Guide](USER_GUIDE.md) - Usage instructions
- [API Reference](API_REFERENCE.md) - Complete API documentation
- [Testing Guide](TESTING.md) - Testing infrastructure
- [Prometheus Rules](../deploy/prometheus/prometheus-rules.yaml) - Alerting rules
- [Grafana Dashboard](../deploy/grafana/dashboard.json) - Dashboard definition
- [README](../README.md) - Project overview

