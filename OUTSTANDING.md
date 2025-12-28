# Outstanding Items for zen-lock

This document tracks outstanding issues, missing features, and improvements needed for zen-lock.

## Critical Issues

### 1. Duplicate Private Key Loading ✅ FIXED
**Location:** `pkg/webhook/pod_handler.go`
**Status:** Fixed - Private key is now stored in PodHandler struct
**Fixed in:** commit fcfaf56

### 2. AllowedSubjects Validation Not Implemented ✅ FIXED
**Location:** `pkg/webhook/pod_handler.go`
**Status:** Fixed - Added `validateAllowedSubjects` function that checks Pod ServiceAccount against allowed subjects
**Fixed in:** commit fcfaf56

### 3. Missing LastTransitionTime in Conditions ✅ FIXED
**Location:** `pkg/controller/reconciler.go`
**Status:** Fixed - LastTransitionTime is now properly set and updated when condition status changes
**Fixed in:** commit fcfaf56

## Missing Features

### 4. Integration Tests
**Status:** Not implemented
**Priority:** High
**Description:** Need integration tests that:
- Test the full encryption/decryption flow
- Test webhook injection in a test cluster
- Test ephemeral secret cleanup
- Test AllowedSubjects validation (once implemented)

### 5. Environment Variable Injection
**Status:** Mentioned in roadmap, not implemented
**Priority:** Medium
**Description:** Support injecting secrets directly as environment variables instead of files, as mentioned in the roadmap v0.2.0.

### 6. KMS Integration
**Status:** Mentioned in roadmap, not implemented
**Priority:** Low (future)
**Description:** Support AWS KMS, Google KMS, Azure KeyVault for master key storage instead of environment variables.

## Code Quality

### 7. Error Handling for Secret Already Exists ✅ FIXED
**Location:** `pkg/webhook/pod_handler.go:135`
**Status:** Fixed - Now uses `k8serrors.IsAlreadyExists(err)` for proper error checking
**Fixed in:** commit fcfaf56

### 8. Missing Context Timeout ✅ FIXED
**Location:** `pkg/webhook/pod_handler.go`
**Status:** Fixed - Added 10 second timeout context to webhook handler
**Fixed in:** commit fcfaf56

## Documentation

### 9. Security Best Practices Guide
**Status:** Missing
**Priority:** Medium
**Description:** Document:
- How to securely store the private key
- Key rotation procedures
- Multi-tenant considerations
- Network policies for webhook

### 10. Troubleshooting Guide Enhancement
**Status:** Basic version exists
**Priority:** Low
**Description:** Add more common issues and solutions:
- Webhook timeout issues
- Certificate problems
- Namespace selector issues

## Testing

### 11. E2E Tests
**Status:** Not implemented
**Priority:** High
**Description:** End-to-end tests covering:
- Full workflow from encryption to pod injection
- Multiple namespaces
- Error scenarios
- Cleanup verification

### 12. Webhook Unit Tests
**Status:** Not implemented
**Priority:** Medium
**Description:** Unit tests for webhook handler with mocked clients.

## Roadmap Items (v0.2.0+)

### 13. Multi-Tenancy Support
**Status:** Planned for v0.2.0
**Priority:** Low
**Description:** Per-namespace encryption keys.

### 14. Certificate Rotation
**Status:** Planned for v1.0.0
**Priority:** Low
**Description:** Automated rolling updates of keys and re-encryption of existing CRDs.

### 15. Validation Webhook
**Status:** Planned for v1.0.0
**Priority:** Low
**Description:** Ensure the public key used matches the cluster's key.

## Quick Wins (Can be fixed immediately) ✅ ALL COMPLETED

1. ✅ Fix duplicate private key loading
2. ✅ Use proper error checking for `IsAlreadyExists`
3. ✅ Add `LastTransitionTime` to conditions
4. ✅ Add context timeout to webhook handler

## Next Steps

1. **Immediate:** Fix critical issues (#1, #2, #7)
2. **Short-term:** Add integration tests (#4)
3. **Medium-term:** Implement AllowedSubjects validation (#2) and add E2E tests (#11)
4. **Long-term:** KMS integration (#6) and other roadmap items

