# zen-lock Architecture

## Overview

zen-lock implements Zero-Knowledge secret storage for Kubernetes using a mutating admission webhook and ephemeral secrets.

## Architecture Components

### 1. CLI (`cmd/cli`)

The CLI is used by developers to encrypt secrets before committing them to Git.

**Responsibilities:**
- Generate encryption key pairs
- Encrypt secret data
- Decrypt secrets (for debugging)
- Extract public keys

**Flow:**
1. Developer runs `zen-lock encrypt` with public key
2. CLI encrypts secret data using age encryption
3. Outputs ZenLock CRD YAML with encrypted data
4. Developer commits encrypted YAML to Git

### 2. Controller (`cmd/webhook`)

The controller runs as a Kubernetes Deployment and includes:
- Controller reconciler for ZenLock CRDs
- Mutating admission webhook server

**Responsibilities:**
- Reconcile ZenLock CRDs and update status
- Handle Pod admission requests
- Inject secrets into Pods
- Create ephemeral secrets

### 3. Webhook Handler (`pkg/webhook`)

The webhook handler intercepts Pod creation requests.

**Flow:**
1. Pod creation request arrives
2. Check for `zen-lock/inject` annotation
3. Fetch ZenLock CRD
4. Validate AllowedSubjects (if configured)
5. Decrypt secret data
6. Create ephemeral Secret with OwnerReference to Pod
7. Patch Pod to mount secret

### 4. Crypto Library (`pkg/crypto`)

Provides encryption/decryption abstraction.

**Current Implementation:**
- Age encryption (X25519)

**Future:**
- Support for multiple encryption backends
- KMS integration

## Data Flow

```
Developer Machine:
  Plaintext Secret → CLI → Encrypted YAML → Git

Kubernetes Cluster:
  Git → kubectl apply → ZenLock CRD (encrypted) → etcd
  
Pod Creation:
  kubectl apply Pod → API Server → Webhook → Decrypt → Ephemeral Secret → Pod
```

## Security Model

### Zero-Knowledge Principles

1. **At Rest (ZenLock CRD)**: Secrets stored as ciphertext in etcd. The API server cannot read the encrypted data.
2. **At Rest (Ephemeral Secrets)**: Decrypted secrets are stored as standard Kubernetes Secrets in etcd. These are protected by:
   - Encryption at rest (if configured for etcd)
   - RBAC controls
   - OwnerReference-based automatic cleanup
   - Short-lived nature (only exist during Pod lifetime)
3. **In Transit**: Encryption happens client-side before data reaches the cluster
4. **In Memory**: Decrypted secrets exist as ephemeral Kubernetes Secrets mounted into Pods
5. **Auto-Cleanup**: Secrets deleted when Pod terminates (via OwnerReference set by controller)

**Important**: While the source-of-truth (ZenLock CRD) is encrypted, ephemeral Secrets created by the webhook are standard Kubernetes Secrets. Enable etcd encryption at rest for additional protection.

### Key Management

- Private keys stored in Kubernetes Secrets or external KMS
- Public keys shared with developers
- Keys never committed to version control

## Ephemeral Secrets

Ephemeral secrets are standard Kubernetes Secrets with:
- OwnerReference pointing to the Pod
- Automatic cleanup when Pod is deleted
- Unique name based on Pod UID
- Same namespace as Pod

## Webhook Configuration

The mutating webhook:
- Intercepts Pod CREATE operations
- Uses TLS for secure communication
- Validates AllowedSubjects (if configured)
- Creates ephemeral secrets atomically

## Testing

zen-lock includes comprehensive testing:

- **Unit Tests**: Fast, isolated tests for individual components
- **Integration Tests**: Component interactions using fake clients
  - Encryption/decryption flow
  - Ephemeral secret cleanup
  - AllowedSubjects validation
  - Status updates
- **E2E Tests**: End-to-end tests with envtest
  - Full workflow from encryption to pod injection
  - Pod injection with webhook server
  - AllowedSubjects validation
  - Invalid ciphertext handling

See [TESTING.md](TESTING.md) for details.

## Future Enhancements

- KMS integration for key management (v0.2.0)
- Multi-tenancy support (v0.2.0)
- Environment variable injection (v0.2.0)
- Certificate rotation (v1.0.0)
- Performance optimizations

See [ROADMAP.md](../ROADMAP.md) for planned features.

