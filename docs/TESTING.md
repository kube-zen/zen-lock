# Testing Guide

This document describes the testing infrastructure and how to run tests for zen-lock.

## Overview

zen-lock has three types of tests:

1. **Unit Tests**: Fast, isolated tests for individual components
2. **Integration Tests**: Tests that verify component interactions using fake clients
3. **E2E Tests**: End-to-end tests that require a real Kubernetes cluster (envtest)

## Unit Tests

Unit tests are located in `pkg/*/*_test.go` files and test individual functions and components in isolation.

### Running Unit Tests

```bash
# Run all unit tests
make test-unit

# Run tests for a specific package
go test -v ./pkg/webhook/...

# Run tests with race detection
go test -race ./pkg/...

# Run tests with coverage
go test -coverprofile=coverage.out ./pkg/...
go tool cover -html=coverage.out
```

### Coverage Requirements

- **Minimum**: 40% code coverage (enforced in CI)
- **Target**: 75% coverage (desired but not enforced)
- **PR Requirement**: >65% coverage (as per PR template)
- **Critical paths**: >85% coverage (recommended)

Coverage is checked automatically in CI and will fail if below 40% minimum threshold.

### Current Coverage

- `pkg/errors`: 84.2% coverage ✅
- `pkg/validation`: 100% coverage ✅
- `pkg/controller`: Improved with additional tests for status updates, decryption paths, and error handling ✅ (meets 40% minimum, working toward 75% target)
- `pkg/webhook`: Improved with additional tests for secret data matching, mutation, dry-run, and validation ✅ (meets 40% minimum, working toward 75% target)
- `pkg/crypto`: Comprehensive tests with dynamic key generation ✅ (tests generate keys at runtime)
- **Note**: `pkg/logging` mentioned in old docs - zen-lock uses `zen-sdk/pkg/logging` (not a local package)

## Integration Tests

Integration tests are located in `test/integration/` and test component interactions using fake Kubernetes clients.

### Test Coverage

Integration tests cover:

- ✅ Controller startup and reconciliation
- ✅ ZenLock CRUD operations
- ✅ Status updates
- ✅ Component interactions
- ✅ Full encryption/decryption flow
- ✅ Ephemeral secret cleanup with OwnerReferences
- ✅ AllowedSubjects validation

### Running Integration Tests

```bash
# Run all integration tests
make test-integration

# Run specific integration test
go test -v ./test/integration/... -run TestZenLockCRUD
```

## E2E Tests

E2E tests are located in `test/e2e/` and require a Kubernetes test environment (envtest).

### Prerequisites

```bash
# Install envtest dependencies
go get sigs.k8s.io/controller-runtime/pkg/envtest@latest
```

### Running E2E Tests

```bash
# Run E2E tests
make test-e2e

# Run specific E2E test
go test -v -tags=e2e ./test/e2e/... -run TestZenLockCRUD_E2E
```

### E2E Test Coverage

- ✅ CRD existence and validation
- ✅ ZenLock CRUD operations
- ✅ Pod injection with webhook server
- ✅ AllowedSubjects validation with webhook server
- ✅ Ephemeral secret creation and OwnerReferences
- ✅ Invalid ciphertext handling
- ✅ Controller reconciliation

## Test Structure

### Unit Test Patterns

- ✅ Table-driven tests for multiple scenarios
- ✅ Test error cases
- ✅ Test edge cases
- ✅ Mock external dependencies

### Integration Test Patterns

- ✅ Use fake clients for isolation
- ✅ Test component interactions
- ✅ Verify cleanup and resource management
- ✅ Test error recovery

## Test Best Practices

### Unit Tests

- Test one thing at a time
- Use table-driven tests for multiple scenarios
- Mock external dependencies
- Test error cases
- Test edge cases

### Integration Tests

- Use fake clients for isolation
- Test component interactions
- Verify cleanup and resource management
- Test error recovery

### E2E Tests

- Test full workflows
- Verify end-to-end behavior
- Test with real Kubernetes API
- Clean up resources after tests

## Troubleshooting

### Tests Fail with "scheme not found"

**Solution**: Ensure `securityv1alpha1.AddToScheme(scheme)` is called before creating fake clients.

### Coverage Below Threshold

**Solution**: 
- If coverage is below 40% minimum: Add more test cases, especially for error paths and edge cases (CI will fail)
- If coverage is between 40-75%: Continue improving toward 75% target, but not blocking
- Coverage is checked in `make coverage` which enforces 40% minimum threshold

### E2E Tests Fail

**Solution**: Ensure envtest is properly installed and CRDs are available.

## Running All Tests

```bash
# Run all tests (unit + integration)
make test

# Run with coverage report
make coverage

# Run CI checks (includes tests)
make ci-check
```

## Test Status

### Current Test Coverage

- ✅ **Integration Tests**: Comprehensive coverage including encryption/decryption flow, ephemeral secret cleanup, and AllowedSubjects validation
- ✅ **E2E Tests**: Full end-to-end tests with webhook server, pod injection, and validation
- ✅ **Webhook Unit Tests**: Enhanced with edge cases and error scenarios
- ✅ **Metrics Tests**: All metric functions now have comprehensive tests with value assertions using test registries
- ✅ **Unit Test Coverage**: All packages meet the 40% minimum threshold
  - `pkg/controller`: 61.4% ✅ (above 40% minimum, working toward 75% target)
  - `pkg/webhook`: 47.6% ✅ (above 40% minimum, working toward 75% target)
  - `pkg/crypto`: Comprehensive tests with dynamic key generation ✅ (tests generate keys at runtime)
  - **Note**: `pkg/logging` mentioned in old docs - zen-lock uses `zen-sdk/pkg/logging` (not a local package)

### Test Files

- Integration tests: `test/integration/integration_test.go`
- E2E tests: `test/e2e/e2e_test.go`
- Webhook tests: `pkg/webhook/pod_handler_test.go`
- Controller tests: `pkg/controller/reconciler_test.go`

## See Also

- [User Guide](USER_GUIDE.md) - Usage instructions
- [Architecture](ARCHITECTURE.md) - System architecture
- [API Reference](API_REFERENCE.md) - Complete API documentation
- [README](../README.md) - Project overview

