# Roadmap

This document outlines the planned features and improvements for zen-lock.

## v0.1.0-alpha (Current) ✅

- ✅ CLI encryption/decryption (Age)
- ✅ Mutating Webhook injection
- ✅ Ephemeral Secrets with OwnerReferences
- ✅ GitOps support
- ✅ AllowedSubjects validation
- ✅ Structured logging and error handling
- ✅ Prometheus metrics

## v0.2.0 (Planned)

### KMS Integration
- Support AWS KMS for master key storage
- Support Google Cloud KMS
- Support Azure Key Vault
- Remove need for private key in cluster

### Multi-Tenancy
- Per-namespace encryption keys
- Namespace-level key management
- Cross-namespace secret sharing controls

### Environment Variable Injection
- Support injecting secrets directly as environment variables
- Safer than files for some applications
- Configurable injection method

### Troubleshooting Guide Enhancement
- Add more common issues and solutions:
  - Webhook timeout issues
  - Certificate problems
  - Namespace selector issues

### Enhanced Validation
- Validation webhook for ZenLock CRDs
- Ensure public key matches cluster key
- Validate encrypted data format

## v1.0.0 (Future)

### Certificate Rotation
- Automated rolling updates of keys
- Re-encryption of existing CRDs
- Zero-downtime key rotation

### Advanced Features
- Secret versioning
- Secret rotation policies
- Audit logging integration
- Multi-key support (key rotation)

### Performance
- Webhook caching
- Batch operations
- Performance optimizations

## Integration Strategy

zen-lock's integration model focuses on authoring-time and key custody, not runtime secret fetching.

### Phase 1: Authoring-Time Providers (v0.2.0)

- Vault CLI integration helpers
- 1Password CLI integration helpers
- Generic provider abstraction for custom integrations

**Pattern**: Pull from provider → encrypt → commit ZenLock CRD (no runtime dependency)

### Phase 2: Key Custody Providers (v0.2.0)

- Vault key injection via Vault Agent
- 1Password key injection via 1Password Operator
- AWS KMS, GCP KMS, Azure Key Vault integration
- Generic Kubernetes Secret provider interface

**Pattern**: Store zen-lock private key in external system; fetch at startup (policy-driven)

### Phase 3: Optional Enhancements (v1.0.0+)

- CSI driver alignment (avoid Kubernetes Secret objects)
- Direct volume mounting of encrypted data
- **Note**: Not promised unless explicitly committed to roadmap

**Non-Goal**: Runtime "fetch from provider during admission" (availability/latency blast radius).

See [INTEGRATIONS.md](docs/INTEGRATIONS.md) for detailed integration strategies.

## Long-Term Vision

- Integration with external secret managers (authoring-time and key custody)
- Support for additional encryption algorithms
- Kubernetes-native key management
- Operator for automated management
- Multi-cluster support

## Contributing

If you'd like to contribute to any of these features, please:
1. Check existing issues
2. Open a new issue to discuss
3. Submit a pull request

We welcome contributions!

