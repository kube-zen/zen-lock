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

## Long-Term Vision

- Integration with external secret managers
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

