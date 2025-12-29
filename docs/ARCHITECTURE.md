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
- ZenLock reconciler for ZenLock CRDs
- Secret reconciler for ephemeral Kubernetes Secret lifecycle
- Mutating admission webhook server

**Responsibilities:**
- Reconcile ZenLock CRDs and update status
- Set OwnerReferences on ephemeral Kubernetes Secrets (once Pod UID is available)
- Clean up orphaned Secrets (Pods that don't exist)
- Handle Pod admission requests
- Inject secrets into Pods

**ServiceAccount Model:**
- Controller and webhook run in separate deployments from the same binary image
- Controller uses `zen-lock-controller` ServiceAccount with `zen-lock-controller` ClusterRole
- Webhook uses `zen-lock-webhook` ServiceAccount with `zen-lock-webhook` ClusterRole
- This separation ensures least privilege - each component only has the permissions it needs
- The binary supports `--enable-controller` and `--enable-webhook` flags to run in component-specific mode

**Performance Optimizations:**
- **ZenLock Caching**: Webhook caches ZenLock CRDs (5min TTL, configurable) to reduce API server load
- **Private Key Caching**: Private key loaded once at startup, cached in handler
- **Input Validation**: Comprehensive validation of annotations and mount paths prevents invalid requests
- **Error Sanitization**: Error messages sanitized to prevent information leakage

### 3. Webhook Handler (`pkg/webhook`)

The webhook handler intercepts Pod creation requests.

**Flow:**
1. Pod creation request arrives
2. Check for `zen-lock/inject` annotation
3. Fetch ZenLock CRD
4. Validate AllowedSubjects (if configured)
5. Decrypt secret data
6. Create ephemeral Kubernetes Secret with labels (OwnerReference set by controller)
7. If secret already exists, validate and refresh stale data
8. Patch Pod to mount secret

**Stale-Secret Handling:**
- If a Secret already exists (e.g., Pod name reused), the webhook validates:
  - Secret matches current ZenLock (by label)
  - Secret data matches current decrypted data
- If stale, the webhook refreshes the Secret with current data
- Prevents stale secrets from persisting when Pod names are reused

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
  kubectl apply Pod → API Server → Injection Webhook → Decrypt → Ephemeral Kubernetes Secret → Pod
```

## Security Model

### Zero-Knowledge Principles

**Zero-knowledge applies to the source-of-truth object; runtime delivery necessarily exposes plaintext to the workload and (via Kubernetes Secret) to any principal with Secret read access.**

1. **At Rest (ZenLock CRD - Source-of-Truth)**: Secrets stored as ciphertext in etcd. The API server cannot read the encrypted data.
2. **At Rest (Ephemeral Kubernetes Secret - Runtime Plaintext)**: Decrypted secrets are stored as standard Kubernetes Secrets in etcd. These are protected by:
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
- Labels for tracking (pod name, namespace, ZenLock name)
- OwnerReference pointing to the Pod (set by SecretReconciler)
- Automatic cleanup when Pod is deleted
- Stable name based on namespace and pod name (SHA256 hash)
- Same namespace as Pod

### Secret Lifecycle

1. **Creation**: Webhook creates Secret with labels (Pod UID not yet available)
2. **OwnerReference**: SecretReconciler sets OwnerReference once Pod UID is available
3. **Cleanup**: Kubernetes automatically deletes Secret when Pod is deleted
4. **Orphan Cleanup**: SecretReconciler deletes orphaned Secrets (>1 minute old, Pod not found)

### Stale-Secret Prevention

- Webhook validates existing Secrets on `AlreadyExists` errors
- Checks if Secret matches current ZenLock (by label)
- Refreshes Secret data if ZenLock was updated
- Prevents stale secrets when Pod names are reused

## Webhook Configuration

The injection webhook (admission-time mutation):
- Intercepts Pod CREATE operations
- Uses TLS for secure communication
- Validates AllowedSubjects (if configured)
- Creates ephemeral Kubernetes Secrets atomically

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

## Trade-offs

### Why Kubernetes Secret Objects?

zen-lock uses standard Kubernetes Secrets for runtime injection to maintain compatibility with existing applications and Kubernetes patterns:

- **Compatibility**: Works with any application that reads secrets from files or environment variables
- **Standard Patterns**: Uses well-understood Kubernetes Secret mounting mechanisms
- **RBAC Integration**: Leverages existing Kubernetes RBAC for access control
- **Lifecycle Management**: Benefits from Kubernetes garbage collection via OwnerReference

### Alternative Patterns

For use cases where Kubernetes Secret objects are not desired, consider:

- **Vault Agent Injector**: Sidecar rendering secrets to a shared volume
  - Reference: [HashiCorp Vault Agent Injector](https://developer.hashicorp.com/vault/docs/platform/k8s/injector)
  - Use when: You need Vault's dynamic secrets, policy engine, or audit capabilities

- **Secrets Store CSI Driver**: Mount external secrets stores as volumes
  - Reference: [Secrets Store CSI Driver](https://secrets-store-csi-driver.sigs.k8s.io/)
  - Use when: You need to avoid Kubernetes Secret objects or integrate with cloud secret managers

- **1Password Kubernetes Operator**: Sync 1Password items into Kubernetes Secrets
  - Reference: [1Password Kubernetes Operator](https://developer.1password.com/docs/connect/kubernetes-operator)
  - Use when: You already use 1Password and want automated secret synchronization

See [FAQ.md](FAQ.md) for decision guidance on when to use zen-lock vs alternatives.

