# Contributing to zen-lock

Thank you for your interest in contributing to zen-lock! This document provides guidelines and instructions for contributing.

## Development Setup

### Prerequisites

- Go 1.24 or later
- Kubernetes cluster (for testing)
- kubectl configured
- Docker (for building images)

### Building

```bash
# Build CLI
make build-cli

# Build webhook controller
make build-controller

# Build both
make build
```

### Running Locally

```bash
# Set private key
export ZEN_LOCK_PRIVATE_KEY=$(cat private-key.age)

# Run webhook locally
make run
```

## Code Style

- Follow Go standard formatting: `go fmt ./...`
- Run `go vet ./...` before committing
- Use meaningful variable and function names
- Add comments for exported functions and types

## Testing

```bash
# Run unit tests
make test

# Run with coverage
make coverage
```

## Submitting Changes

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass: `make test`
6. Commit your changes: `git commit -m "Add feature X"`
7. Push to your fork: `git push origin feature/my-feature`
8. Open a Pull Request

## Pull Request Guidelines

- Provide a clear description of the changes
- Reference any related issues
- Ensure CI checks pass
- Request review from maintainers

## Code Review

All code must be reviewed before merging. Be responsive to feedback and make requested changes promptly.

## Questions?

Open an issue or start a discussion on GitHub.

