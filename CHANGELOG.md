# Changelog

All notable changes to zen-lock will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Core packages: errors, logging, validation, metrics
- AllowedSubjects validation in webhook
- Context timeout for webhook handler
- LastTransitionTime tracking in conditions
- Quality gap analysis documentation
- Project structure documentation
- Security policy
- Maintainer and governance documentation

### Fixed
- Duplicate private key loading (now stored in struct)
- Error handling using k8serrors.IsAlreadyExists
- Proper error context throughout codebase

## [0.1.0-alpha] - 2025-01-XX

### Added
- Initial release
- CLI with keygen, encrypt, decrypt, pubkey commands
- ZenLock CRD with v1alpha1 API
- Mutating webhook for Pod secret injection
- Controller reconciler for status management
- Age encryption support
- Ephemeral secrets with OwnerReferences
- GitOps support
- Examples and documentation
- Install script
- CI workflow

[Unreleased]: https://github.com/kube-zen/zen-lock/compare/v0.1.0-alpha...HEAD
[0.1.0-alpha]: https://github.com/kube-zen/zen-lock/releases/tag/v0.1.0-alpha

