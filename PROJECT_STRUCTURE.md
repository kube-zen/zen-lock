# zen-lock Project Structure

## Overview

zen-lock is a Kubernetes-native secret manager that implements Zero-Knowledge secret storage. ZenLock CRD is ciphertext in etcd; runtime injection uses short-lived Kubernetes Secrets subject to RBAC and (recommended) etcd encryption-at-rest.

## Project Goals

1. **Zero-Knowledge**: ZenLock CRDs store only ciphertext (source-of-truth). API server/etcd cannot read the ZenLock CRD payload (ciphertext). Zero-knowledge applies to the ZenLock CRD (ciphertext). Runtime delivery exposes plaintext to the workload and to any principal that can read the generated Kubernetes Secret.
2. **Ephemeral Lifecycle**: Decrypted secrets are stored in an ephemeral Kubernetes Secret for Pod consumption and are cleaned up when Pods terminate (plus orphan TTL cleanup).
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
│   │   └── security.kube-zen.io/
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
│   │       └── security.kube-zen.io_zenlocks.yaml
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
- Includes CRD types in `pkg/apis/security.kube-zen.io/v1alpha1/`

### `/pkg/apis/security.kube-zen.io/v1alpha1`
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

**Zero-Knowledge Definition**: Zero-knowledge applies to the ZenLock CRD (ciphertext). API server/etcd cannot read the ZenLock CRD payload (ciphertext). Runtime delivery is plaintext by design (Kubernetes Secret + volume mount). Runtime delivery exposes plaintext to the workload and to any principal that can read the generated Kubernetes Secret.

### Key Principles

- ✅ Source-of-truth (ZenLock CRD) never stored in plaintext (runtime Secret is plaintext by design)
- ✅ Ephemeral Kubernetes Secrets with automatic cleanup
- ✅ Client-side encryption
- ✅ Kubernetes-native patterns
- ✅ No external dependencies

**Limitations / Non-goals**: See [docs/FAQ.md](docs/FAQ.md) for positioning, limitations, and non-goals. See [SECURITY.md](SECURITY.md#known-security-gaps--trade-offs) for known security gaps and trade-offs.

## Version

Current version: **0.1.0-alpha**

**Production Readiness**: zen-lock is production-ready for operational concerns (HA, leader election, observability, reliability). Some security features are planned for future versions (KMS integration, automated key rotation, multi-tenancy). See [ROADMAP.md](../ROADMAP.md) for details.

