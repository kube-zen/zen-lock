# Go 1.25 Optimization Opportunities

This document outlines optimization opportunities for zen-lock now that we're using Go 1.25.

## Go 1.25 Features

### 1. Improved Slice Allocation
Go 1.25 compiler now allocates slice backing stores on the stack more frequently, leading to performance improvements. We can leverage this by:
- Pre-allocating slices with known capacity using `make([]T, 0, capacity)`
- Using capacity hints in `append()` operations

### 2. Container-Aware GOMAXPROCS
On Linux systems, Go 1.25's runtime automatically adjusts `GOMAXPROCS` based on CPU bandwidth limits from cgroups. This is automatic and requires no code changes, but improves resource utilization in Kubernetes.

### 3. Enhanced Garbage Collector (Experimental)
Go 1.25 includes an experimental "Green Tea" garbage collector optimized for small object management, potentially reducing GC overhead by 10-40%. Can be enabled with `GOEXPERIMENT=greenteagc` at build time.

### 4. DWARF5 Debug Information
Enabled by default, reduces debug data size and linking times. No code changes needed.

## Identified Optimization Opportunities

### 1. Cache Cleanup Efficiency
**Location**: `pkg/webhook/cache.go:138`

**Current Implementation**:
```go
for key, entry := range c.cache {
    if now.After(entry.expiresAt) {
        delete(c.cache, key)
    }
}
```

**Optimization**: Collect expired keys first, then delete in batch to avoid modifying map during iteration (though Go allows this, collecting first is more efficient).

### 2. Slice Pre-allocation
**Locations**:
- `pkg/crypto/age.go:30` - Already uses capacity hint ✅
- `pkg/crypto/age.go:96` - Map allocation could use size hint
- `pkg/webhook/pod_handler.go:300` - Map allocation could use size hint

### 3. Map Pre-allocation
**Locations**:
- `pkg/webhook/pod_handler.go:145` - `make(map[string]string)` could use size hint if known
- `pkg/webhook/pod_handler.go:300` - `make(map[string][]byte)` could use size hint

### 4. Context Usage
**Current**: Most code uses `context.Background()` in tests, which is fine.
**Production**: Already uses `context.WithTimeout` appropriately ✅

## Recommended Optimizations

### High Priority

1. **Pre-allocate maps with known size**
   - When decrypting maps, we know the size - use `make(map[string][]byte, len(encryptedData))`
   - When creating label maps, estimate size

2. **Optimize cache cleanup**
   - Collect expired keys first, then delete in batch
   - Reduces map modification overhead

### Medium Priority

3. **Consider experimental features** (for testing)
   - Test `GOEXPERIMENT=greenteagc` in staging
   - Monitor GC pause times and throughput

4. **Slice capacity hints**
   - Review all slice allocations for capacity hints
   - Most already have them ✅

### Low Priority

5. **Documentation**
   - Document container-aware GOMAXPROCS behavior
   - Document experimental features for future consideration

## Implementation Status

- ✅ Slice capacity hints already used in most places
- ⚠️ Map size hints could be added where size is known
- ⚠️ Cache cleanup could be optimized
- ✅ Context usage is appropriate
- ✅ No deprecated patterns found

## Performance Impact

Expected improvements:
- **Map pre-allocation**: 5-10% reduction in allocations for decryption operations
- **Cache cleanup optimization**: Minimal impact, but cleaner code
- **Container-aware GOMAXPROCS**: Automatic, no code changes needed
- **Green Tea GC** (if enabled): 10-40% reduction in GC overhead (experimental)

## Next Steps

1. Implement map size hints where size is known
2. Optimize cache cleanup loop
3. Test experimental Green Tea GC in staging environment
4. Monitor performance metrics after optimizations

