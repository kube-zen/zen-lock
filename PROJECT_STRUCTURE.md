# zen-lock Project Structure

## Overview

zen-lock is a Kubernetes-native secret manager that implements Zero-Knowledge secret storage. It ensures your secrets are never stored in plaintext in etcd and never visible via kubectl.

## Project Goals

1. **Zero-Knowledge**: Secrets are ciphertext in etcd. The API server cannot read them.
2. **Ephemeral Lifecycle**: Decrypted secrets exist only for the lifetime of the Pod.
3. **GitOps Ready**: Encrypted manifests can be safely committed to Git.
4. **Kubernetes-Native**: Uses standard CRDs and Mutating Webhooks. No external databases.

## Project Structure

```
zen-lock/
├── cmd/
│   ├── cli/                      # CLI binary (zen-lock)
│   │   ├── main.go              # CLI entrypoint
│   │   ├── keygen.go            # Key generation command
│   │   ├── encrypt.go           # Encryption command
│   │   ├── decrypt.go           # Decryption command
│   │   └── pubkey.go            # Public key extraction
│   └── webhook/                  # Webhook controller binary
│       └── main.go              # Controller entrypoint
├── pkg/
│   ├── apis/                     # CRD type definitions
│   │   └── security.zen.io/
│   │       └── v1alpha1/
│   │           ├── groupversion_info.go
│   │           ├── zenlock_types.go
│   │           └── zz_generated.deepcopy.go
│   ├── controller/               # Controller implementation
│   │   ├── reconciler.go        # ZenLock reconciler
│   │   └── metrics/             # Prometheus metrics
│   │       └── metrics.go
│   ├── crypto/                   # Encryption library
│   │   ├── interface.go        # Encryption interface
│   │   └── age.go              # Age encryption implementation
│   ├── errors/                   # Structured error handling
│   │   └── errors.go
│   ├── logging/                  # Structured logging
│   │   └── logger.go
│   ├── validation/               # Validation utilities
│   │   └── validator.go
│   └── webhook/                  # Webhook implementation
│       ├── pod_handler.go       # Pod mutation handler
│       └── webhook.go           # Webhook setup
├── config/
│   ├── crd/                      # CRD definitions
│   │   └── bases/
│   │       └── security.zen.io_zenlocks.yaml
│   ├── rbac/                     # RBAC manifests
│   │   └── role.yaml
│   └── webhook/                  # Webhook configuration
│       ├── manifests.yaml
│       └── certificate.yaml
├── examples/                     # Example manifests
│   ├── secret.yaml
│   ├── deployment.yaml
│   └── README.md
├── test/                         # Tests
│   ├── unit/                    # Unit tests
│   ├── integration/             # Integration tests
│   └── e2e/                     # E2E tests
├── docs/                         # Documentation
│   ├── API_REFERENCE.md
│   ├── ARCHITECTURE.md
│   ├── SECURITY.md
│   └── USER_GUIDE.md
├── Dockerfile
├── Makefile
├── go.mod
├── LICENSE
├── NOTICE
├── README.md
└── PROJECT_STRUCTURE.md          # This file
```

## Directory Purposes

### `/cmd`
**Purpose**: Main application entry points

- `cmd/cli`: CLI binary for encryption/decryption operations
- `cmd/webhook`: Controller binary for webhook server
- Keep minimal - just wiring
- Logic lives in `pkg/`

### `/pkg`
**Purpose**: All reusable code

- Well-organized packages
- Business logic
- Can be imported by other projects
- Includes CRD types in `pkg/apis/security.zen.io/v1alpha1/`

### `/pkg/apis/security.zen.io/v1alpha1`
**Purpose**: CRD type definitions

- ZenLock CRD types
- Type constants
- Deep copy methods
- API group/version registration

### `/pkg/controller`
**Purpose**: Controller implementation

- Main reconciliation logic
- Metrics and events
- Status management

### `/pkg/webhook`
**Purpose**: Mutating webhook implementation

- Pod admission webhook
- Secret injection logic
- Validation

### `/pkg/crypto`
**Purpose**: Encryption library

- Encryption/decryption interface
- Age encryption implementation
- Future: Support for other encryption backends

### `/config`
**Purpose**: Kubernetes manifests

- CRD definitions
- Deployment manifests
- RBAC configuration
- Webhook configuration

### `/examples`
**Purpose**: Working examples

- Example ZenLock manifests
- Example Pod deployments
- Tutorial configs

## Development Status

### Current Status: Alpha (v0.1.0-alpha) ✅

- ✅ CLI encryption/decryption (Age)
- ✅ Mutating Webhook injection
- ✅ Ephemeral Secrets with OwnerReferences
- ✅ GitOps support
- ✅ AllowedSubjects validation
- ✅ Structured logging and error handling
- ✅ Prometheus metrics
- ✅ Project structure and build system

### Next Steps

- ⬜ Integration tests
- ⬜ E2E tests
- ⬜ KMS Integration (AWS KMS, Google KMS)
- ⬜ Environment variable injection
- ⬜ Multi-tenancy support
- ⬜ Certificate rotation

## Design Philosophy

**Zero-Knowledge First**: zen-lock prioritizes security and zero-knowledge principles above all else.

### Key Principles

- ✅ Secrets never stored in plaintext
- ✅ Ephemeral secrets with automatic cleanup
- ✅ Client-side encryption
- ✅ Kubernetes-native patterns
- ✅ No external dependencies

## Version

Current version: **0.1.0-alpha**

Even though this is production-grade architecture, we maintain the alpha version to indicate this is early-stage software that may have breaking changes.

