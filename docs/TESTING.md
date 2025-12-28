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

- **Minimum**: 75% code coverage
- **Target**: >80% coverage
- **Critical paths**: >85% coverage

Coverage is checked automatically in CI and will fail if below 75%.

### Current Coverage

- `pkg/errors`: 84.2% coverage ✅
- `pkg/validation`: 100% coverage ✅
- `pkg/controller`: 61.4% coverage ⚠️
- `pkg/logging`: 57.4% coverage ⚠️
- `pkg/webhook`: 47.6% coverage ⚠️
- `pkg/crypto`: 0% coverage (requires actual keys)

## Integration Tests

Integration tests are located in `test/integration/` and test component interactions using fake Kubernetes clients.

### Test Coverage

Integration tests cover:

- ✅ Controller startup and reconciliation
- ✅ ZenLock CRUD operations
- ✅ Status updates
- ✅ Component interactions

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
- ⏳ Pod injection (requires webhook server)
- ⏳ AllowedSubjects validation (requires webhook server)

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

**Solution**: Add more test cases, especially for error paths and edge cases.

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

