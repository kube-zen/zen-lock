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

### 4. Integration Tests ✅ ENHANCED
**Status:** Enhanced with comprehensive tests
**Priority:** High
**Description:** Integration tests now cover:
- ✅ Full encryption/decryption flow
- ✅ Ephemeral secret cleanup with OwnerReferences
- ✅ AllowedSubjects validation
- ✅ ZenLock status updates
**Enhanced in:** Latest update

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

### 9. Security Best Practices Guide ✅ COMPLETED
**Status:** Completed
**Priority:** Medium
**Description:** Comprehensive guide created at `docs/SECURITY_BEST_PRACTICES.md` covering:
- ✅ How to securely store the private key
- ✅ Key rotation procedures
- ✅ Multi-tenant considerations
- ✅ Network policies for webhook
- ✅ Access control and RBAC
- ✅ Audit and monitoring
- ✅ Compliance considerations
**Completed in:** Latest update

### 10. Troubleshooting Guide Enhancement
**Status:** Basic version exists
**Priority:** Low
**Description:** Add more common issues and solutions:
- Webhook timeout issues
- Certificate problems
- Namespace selector issues

### 10a. User Guide ✅ COMPLETED
**Status:** Completed
**Priority:** High
**Description:** Comprehensive user guide created at `docs/USER_GUIDE.md` covering:
- ✅ Installation instructions
- ✅ Getting started guide
- ✅ Key management
- ✅ Encrypting and deploying secrets
- ✅ Injecting secrets into Pods
- ✅ AllowedSubjects usage
- ✅ Troubleshooting
- ✅ Best practices
**Completed in:** Latest update

### 10b. RBAC Documentation ✅ COMPLETED
**Status:** Completed
**Priority:** Medium
**Description:** RBAC documentation created at `docs/RBAC.md` covering:
- ✅ ClusterRole permissions
- ✅ ServiceAccount configuration
- ✅ User permissions
- ✅ Security considerations
- ✅ Troubleshooting RBAC
**Completed in:** Latest update

## Testing

### 11. E2E Tests ✅ IMPLEMENTED
**Status:** Comprehensive E2E tests implemented
**Priority:** High
**Description:** E2E tests cover:
- ✅ Full workflow from encryption to pod injection
- ✅ ZenLock CRUD operations
- ✅ Controller reconciliation
- ✅ Pod injection with volume mounts
- ✅ Ephemeral secret creation and OwnerReferences
- ✅ AllowedSubjects validation
- ✅ Invalid ciphertext handling
**Location:** `test/e2e/e2e_test.go`

### 12. Webhook Unit Tests ✅ ENHANCED
**Status:** Enhanced with comprehensive tests
**Priority:** Medium
**Description:** Unit tests now cover:
- ✅ ZenLock not found scenarios
- ✅ AllowedSubjects denial scenarios
- ✅ Custom mount paths
- ✅ Multiple containers
- ✅ Error handling
**Enhanced in:** Latest update

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

1. **Completed:** ✅ Fixed critical issues (#1, #2, #7)
2. **Completed:** ✅ Enhanced integration tests (#4)
3. **Completed:** ✅ Enhanced webhook unit tests (#12)
4. **Completed:** ✅ Added comprehensive documentation (#9, #10a, #10b)
5. **Short-term:** Environment variable injection (#5) - v0.2.0
6. **Medium-term:** KMS integration (#6) - v0.2.0
7. **Long-term:** Multi-tenancy support (#13), Certificate rotation (#14), Validation webhook (#15) - v1.0.0

## Recent Updates

- ✅ Enhanced integration tests with encryption/decryption flow, ephemeral secret cleanup, and AllowedSubjects validation
- ✅ Enhanced webhook unit tests with edge cases and error scenarios
- ✅ Created comprehensive User Guide (`docs/USER_GUIDE.md`)
- ✅ Created RBAC documentation (`docs/RBAC.md`)
- ✅ Created Security Best Practices guide (`docs/SECURITY_BEST_PRACTICES.md`)

