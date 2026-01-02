# Metrics, Alerts, Dashboard, and Tests Review

## Summary

Comprehensive review of zen-lock metrics, alert rules, Grafana dashboard, and test coverage.

---

## Issues Found

### 1. **CRITICAL: Cache Metrics Never Updated**
**Severity**: High  
**Location**: `pkg/controller/metrics/metrics.go`

**Issue**: 
- `CacheSizeGauge` and `CacheHitRateGauge` are defined but `UpdateCacheMetrics()` is never called
- Cache metrics are tracked via counters (`zenlock_cache_hits_total`, `zenlock_cache_misses_total`) but gauge metrics are not updated

**Impact**: 
- Dashboard queries for `zenlock_cache_size` and `zenlock_cache_hit_rate` will always return 0
- Cache performance cannot be monitored via gauges

**Recommendation**:
- Call `UpdateCacheMetrics()` periodically in cache manager or when cache operations occur
- Add background goroutine to update cache metrics every 30-60 seconds

**Fix**:
```go
// In pkg/webhook/cache.go or cache_manager.go
func (c *ZenLockCache) updateMetrics() {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    size := len(c.cache)
    hits := c.hits  // Need to track hits/misses in cache struct
    misses := c.misses
    metrics.UpdateCacheMetrics(size, hits, misses)
}
```

---

### 2. **Algorithm Metrics Never Recorded**
**Severity**: Medium  
**Location**: Multiple files

**Issue**:
- `RecordAlgorithmUsage()` and `RecordAlgorithmError()` are defined but never called
- Algorithm metrics (`zenlock_algorithm_usage_total`, `zenlock_algorithm_errors_total`) will always be 0

**Impact**:
- Cannot track algorithm usage distribution
- Cannot monitor algorithm-related errors
- Dashboard panels for algorithm metrics will show no data

**Recommendation**:
- Record algorithm usage in `pkg/crypto/age.go` during encrypt/decrypt operations
- Record algorithm errors when validation fails in `pkg/webhook/zenlock_validator.go`

**Fix Locations**:
1. `pkg/crypto/age.go`: Record usage on encrypt/decrypt
2. `pkg/webhook/zenlock_validator.go`: Record errors on validation failures

---

### 3. **Missing Metric Registration for Gauges**
**Severity**: Medium  
**Location**: `pkg/controller/metrics/metrics.go:129-142`

**Issue**:
- `CacheSizeGauge` and `CacheHitRateGauge` use `prometheus.NewGauge()` instead of `promauto.NewGauge()`
- They are not automatically registered with the default registry

**Impact**:
- Metrics may not be exposed if not manually registered

**Recommendation**:
- Use `promauto.NewGauge()` for consistency with other metrics
- Or ensure manual registration in main.go

**Fix**:
```go
CacheSizeGauge = promauto.NewGauge(
    prometheus.GaugeOpts{
        Name: "zenlock_cache_size",
        Help: "Current number of entries in the ZenLock cache",
    },
)

CacheHitRateGauge = promauto.NewGauge(
    prometheus.GaugeOpts{
        Name: "zenlock_cache_hit_rate",
        Help: "Cache hit rate (hits / (hits + misses))",
    },
)
```

---

### 4. **Test Coverage Issues**
**Severity**: Low  
**Location**: `pkg/controller/metrics/metrics_test.go`, `metrics_cache_test.go`

**Issues**:
- Tests only verify functions don't panic, don't verify metric values
- No tests for metric label values
- No tests for histogram bucket behavior
- No tests for gauge updates

**Recommendation**:
- Use `prometheus.NewRegistry()` and `promtest` package for proper metric testing
- Verify metric values and labels are correct
- Test histogram bucket distribution

**Example Test**:
```go
func TestRecordReconcile_Values(t *testing.T) {
    reg := prometheus.NewRegistry()
    // Create metrics with custom registry
    // Record metrics
    // Gather and verify values
}
```

---

### 5. **Alert Rule Issues**

#### 5.1 Missing Cache Metrics Alerts
**Severity**: Low  
**Location**: `deploy/prometheus/prometheus-rules.yaml`

**Issue**: No alerts for:
- Low cache hit rate (< 50%)
- Cache size growing too large
- Cache performance degradation

**Recommendation**: Add alerts:
```yaml
- alert: ZenLockLowCacheHitRate
  expr: zenlock_cache_hit_rate < 0.5
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "Low cache hit rate detected"
    description: "Cache hit rate is {{ $value | humanizePercentage }}, indicating inefficient caching"
```

#### 5.2 Alert Thresholds May Be Too High
**Severity**: Low  
**Location**: `deploy/prometheus/prometheus-rules.yaml`

**Issues**:
- `ZenLockHighReconciliationErrorRate`: > 5 errors/sec may be too high for small clusters
- `ZenLockWebhookInjectionFailures`: > 2 failures/sec may miss issues in low-traffic environments
- Consider making thresholds configurable or environment-specific

---

### 6. **Dashboard Issues**

#### 6.1 Missing Cache Size Panel
**Severity**: Low  
**Location**: `deploy/grafana/dashboard.json`

**Issue**: Dashboard shows cache hit rate but not cache size

**Recommendation**: Add panel for `zenlock_cache_size` gauge

#### 6.2 Dashboard Uses Unregistered Gauges
**Severity**: Medium  
**Location**: `deploy/grafana/dashboard.json:321-344`

**Issue**: Panel ID 12 queries `zenlock_cache_hit_rate` which may not be exposed (see Issue #3)

**Impact**: Panel will show "No data"

---

### 7. **Metrics Documentation Gaps**
**Severity**: Low  
**Location**: `docs/METRICS.md`

**Issues**:
- Missing documentation for `zenlock_cache_size` gauge
- Missing documentation for `zenlock_cache_hit_rate` gauge
- Missing examples for algorithm metrics (since they're not used)

**Recommendation**: Update documentation to reflect actual usage

---

## Positive Findings

### ✅ Good Practices

1. **Comprehensive Metric Coverage**: Metrics cover all major operations (reconcile, webhook, decryption, cache)
2. **Proper Labeling**: Metrics use appropriate labels (namespace, name, result) for filtering
3. **Histogram Buckets**: Exponential buckets are appropriate for duration measurements
4. **Alert Coverage**: Alerts cover critical failure scenarios
5. **Dashboard Layout**: Dashboard is well-organized with logical panel grouping

### ✅ Metric Recording

- Reconcile metrics: ✅ Properly recorded
- Webhook injection metrics: ✅ Properly recorded  
- Decryption metrics: ✅ Properly recorded
- Cache hit/miss metrics: ✅ Properly recorded
- Validation failure metrics: ✅ Properly recorded

---

## Recommendations Priority

### Priority 1 (Fix Immediately)
1. Fix cache gauge registration (Issue #3)
2. Implement cache metrics updates (Issue #1)

### Priority 2 (Fix Soon)
3. Record algorithm metrics (Issue #2)
4. Fix dashboard cache panel (Issue #6.2)

### Priority 3 (Nice to Have)
5. Improve test coverage (Issue #4)
6. Add cache alerts (Issue #5.1)
7. Update documentation (Issue #7)

---

## Action Items

- [ ] Fix `CacheSizeGauge` and `CacheHitRateGauge` registration
- [ ] Implement periodic cache metrics updates
- [ ] Add algorithm metric recording in crypto package
- [ ] Add algorithm error recording in validator
- [ ] Improve metric tests with proper registry
- [ ] Add cache size panel to dashboard
- [ ] Add cache performance alerts
- [ ] Update METRICS.md documentation

