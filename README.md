# zen-lock

zen-lock is a Kubernetes-native secret manager that implements Zero-Knowledge secret storage. It ensures your secrets are encrypted at rest in the source-of-truth (ZenLock CRD) and only decrypted ephemerally for Pod injection.

## Features

- **Zero-Knowledge**: Secrets are ciphertext in etcd. The API server cannot read them.
- **Ephemeral Lifecycle**: Decrypted secrets exist only for the lifetime of the Pod.
- **GitOps Ready**: Encrypted manifests can be safely committed to Git.
- **Kubernetes-Native**: Uses standard CRDs and Mutating Webhooks. No external databases.
- **Age Encryption**: Uses modern, easy-to-use encryption (age) by default.
- **AllowedSubjects**: Restrict secret access to specific ServiceAccounts.
- **Comprehensive Testing**: Full integration and E2E test coverage.
- **Production-Ready Operations**: High availability, leader election, comprehensive observability (metrics, alerts, dashboards), least-privilege RBAC, orphan cleanup, stale-secret prevention.

## Scope and Non-Goals

zen-lock is **not**:
- A centralized secrets platform (no auth methods, policy engine, audit devices)
- Dynamic secrets/leased credentials (DB/cloud rotations)
- A general "sync secrets from external providers" operator
- A hard security boundary against cluster-admin (RBAC/etcd encryption required)

**Use zen-lock when**: Static secrets + GitOps is the goal.  
**Use alternatives when**: You need dynamic secrets, centralized policy, or to avoid Kubernetes Secret objects.

See [FAQ](docs/FAQ.md) for detailed positioning and [INTEGRATIONS.md](docs/INTEGRATIONS.md) for integration strategies.

## Integrations

zen-lock supports integration with external secret managers through two modes:

1. **Authoring-Time**: Pull from provider → encrypt → commit ZenLock CRD (no runtime dependency)
2. **Key Custody**: Store zen-lock private key in external system; fetch at startup (policy-driven)

**Non-Goal**: Runtime "fetch from provider during admission" (availability/latency blast radius).

See [INTEGRATIONS.md](docs/INTEGRATIONS.md) for detailed integration strategies with Vault, 1Password, and other providers.

## Quick Start

### 1. Generate Keys

```bash
zen-lock keygen --output private-key.age
```

This creates a private key file and displays the public key.

### 2. Export Public Key

```bash
zen-lock pubkey --input private-key.age > public-key.age
```

Share `public-key.age` with your team.

### 3. Encrypt a Secret

Create a file `secret.yaml`:

```yaml
metadata:
  name: db-credentials
stringData:
  DB_USER: "admin"
  DB_PASS: "SuperSecret123!"
```

Encrypt it:

```bash
zen-lock encrypt --pubkey $(cat public-key.age) --input secret.yaml --output encrypted-secret.yaml
```

### 4. Deploy to Cluster

```bash
kubectl apply -f encrypted-secret.yaml
```

### 5. Inject into Pod

Create a deployment with the injection annotation:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  template:
    metadata:
      annotations:
        zen-lock/inject: "db-credentials"
        zen-lock/mount-path: "/etc/config"
    spec:
      containers:
      - name: app
        image: nginx
        volumeMounts:
        - name: zen-secrets
          mountPath: /etc/config
```

## Installation

### CLI

**macOS (Homebrew):**
```bash
brew tap kube-zen/tap
brew install zen-lock
```

**Linux:**
```bash
curl -sSL https://raw.githubusercontent.com/kube-zen/zen-lock/main/install.sh | bash
```

**From Source:**
```bash
git clone https://github.com/kube-zen/zen-lock.git
cd zen-lock
make build-cli
sudo mv bin/zen-lock /usr/local/bin/
```

### Controller

**Using kubectl:**
```bash
kubectl apply -f config/crd/bases/
kubectl apply -f config/rbac/
kubectl apply -f config/webhook/
```

**Using Helm:**

The chart is available via GitHub Pages and Artifact Hub:

```bash
# Add the repository
helm repo add zen-lock https://kube-zen.github.io/zen-lock
helm repo update

# Install zen-lock
helm install zen-lock zen-lock/zen-lock \
  --namespace zen-lock-system \
  --create-namespace
```

- **GitHub Pages**: https://kube-zen.github.io/zen-lock
- **Artifact Hub**: https://artifacthub.io/packages/helm/zen-lock/zen-lock

See [Helm Repository Documentation](docs/HELM_REPOSITORY.md) for details.

## Configuration

The controller needs access to the Private Key to decrypt secrets.

**Option A: Environment Variable (Basic)**
Edit the Deployment to add the private key:

```yaml
env:
  - name: ZEN_LOCK_PRIVATE_KEY
    valueFrom:
      secretKeyRef:
        name: zen-lock-master-key
        key: key.txt
```

**Option B: AWS KMS (Planned)**
Set the `--kms-key-id` flag in the controller arguments.

## CLI Reference

### `zen-lock keygen`
Generates a new encryption key pair.

```bash
zen-lock keygen --output ~/.zen-lock/key.age
```

### `zen-lock pubkey`
Extracts the public key from a private key file.

```bash
zen-lock pubkey --input ~/.zen-lock/key.age > pubkey.txt
```

### `zen-lock encrypt`
Encrypts a YAML file containing secret data.

```bash
zen-lock encrypt \
  --pubkey age1q3... \
  --input plain-secret.yaml \
  --output encrypted-zenlock.yaml
```

### `zen-lock decrypt`
Decrypts a ZenLock CRD file back to plain text (debug only).

```bash
zen-lock decrypt \
  --privkey ~/.zen-lock/key.age \
  --input encrypted-zenlock.yaml \
  --output plain-secret.yaml
```

## API Reference

### ZenLock CRD

```yaml
apiVersion: security.kube-zen.io/v1alpha1
kind: ZenLock
metadata:
  name: example-secret
  namespace: production
spec:
  encryptedData:
    USERNAME: <ciphertext>
    API_KEY: <ciphertext>
  algorithm: age
  allowedSubjects:
  - kind: ServiceAccount
    name: backend-app
    namespace: production
```

## Development

### Building

```bash
# Build CLI
make build-cli

# Build Controller
make build-controller

# Build Docker Image
make build-image
```

### Running Locally

```bash
# Set private key
export ZEN_LOCK_PRIVATE_KEY=$(cat private-key.age)

# Run webhook locally
make run
```

## Production Readiness

zen-lock is **production-ready** for operational concerns:

- ✅ **High Availability**: Leader election, separate controller/webhook deployments, graceful shutdown
- ✅ **Observability**: Comprehensive Prometheus metrics, Grafana dashboards, alerting rules
- ✅ **Reliability**: Orphan cleanup, stale-secret prevention, error handling, input validation
- ✅ **Security**: Least-privilege RBAC, separate ServiceAccounts, zero-knowledge encryption
- ✅ **Testing**: Full integration and E2E test coverage

**Security Features Status**:

- ✅ **Current (v0.1.0-alpha)**: Zero-knowledge encryption, AllowedSubjects, ephemeral secrets, automatic cleanup
- 🔄 **Planned (v0.2.0)**: KMS integration (AWS KMS, GCP KMS, Azure Key Vault), multi-tenancy (per-namespace keys), environment variable injection
- 🔄 **Planned (v1.0.0)**: Automated key rotation, secret versioning, audit logging integration

See [ROADMAP.md](ROADMAP.md) for detailed feature plans.

## Security Model

zen-lock implements Zero-Knowledge encryption with the following security properties:

- **At Rest (ZenLock CRD)**: The ZenLock CRD stored in etcd contains only unreadable ciphertext. The API server cannot read the encrypted data.
- **At Rest (Ephemeral Secrets)**: Decrypted secrets are stored as standard Kubernetes Secrets in etcd. These are protected by:
  - Encryption at rest (if configured for etcd)
  - RBAC controls
  - OwnerReference-based automatic cleanup
  - Short-lived nature (only exist during Pod lifetime)
- **In Transit**: Encryption happens on the developer's machine before data reaches the cluster.
- **In Memory**: The decrypted value exists as a standard Kubernetes Secret mounted into the Pod.
- **Auto-Cleanup**: By setting the OwnerReference of the decrypted secret to the Pod, Kubernetes guarantees that the secret is deleted when the Pod is removed.

**Important**: The source-of-truth (ZenLock CRD) is encrypted and never stored in plaintext. However, ephemeral Secrets created by the webhook are standard Kubernetes Secrets containing decrypted data. **Ephemeral Secrets are standard Kubernetes Secrets and can be read by principals with Secret read access; treat RBAC/etcd encryption as mandatory controls.** Enable etcd encryption at rest for additional protection of ephemeral Secrets.

## Documentation

Comprehensive documentation is available in the `docs/` directory:

- **[User Guide](docs/USER_GUIDE.md)** - Complete guide for using zen-lock
- **[API Reference](docs/API_REFERENCE.md)** - Complete API documentation
- **[Architecture](docs/ARCHITECTURE.md)** - System architecture and design
- **[RBAC](docs/RBAC.md)** - RBAC permissions and configuration
- **[Security Best Practices](docs/SECURITY_BEST_PRACTICES.md)** - Security guidelines
- **[Testing Guide](docs/TESTING.md)** - Testing infrastructure and coverage
- **[Metrics](docs/METRICS.md)** - Prometheus metrics documentation
- **[Helm Repository](docs/HELM_REPOSITORY.md)** - Helm chart repository setup and usage

## Troubleshooting

### Pod is stuck in ContainerCreating

Check webhook logs:
```bash
kubectl logs -n zen-lock-system deployment/zen-lock-webhook
```

Verify private key is set:
```bash
kubectl get deployment zen-lock-webhook -n zen-lock-system -o yaml | grep ZEN_LOCK_PRIVATE_KEY
```

### Changes not reflected in running Pods

Secrets are injected at Pod creation time. You must delete and recreate the Pod to get updated secrets.

### Webhook Denial

If Pod creation is denied:
1. Check AllowedSubjects: Verify the Pod's ServiceAccount is in the allowed list
2. Check webhook logs: Look for denial reasons
3. Verify ZenLock exists: Ensure the ZenLock CRD exists in the namespace

For more troubleshooting tips, see the [User Guide](docs/USER_GUIDE.md#troubleshooting).

## Contributing

We welcome contributions! Please see CONTRIBUTING.md for details.

## License

Apache License 2.0

## Support

- **Issues**: [GitHub Issues](https://github.com/kube-zen/zen-lock/issues)
- **Discussions**: [GitHub Discussions](https://github.com/kube-zen/zen-lock/discussions)
- **Documentation**: See [docs/](docs/) directory for comprehensive guides

