# User Guide

This guide provides comprehensive instructions for using zen-lock to manage secrets in Kubernetes.

## Table of Contents

1. [Installation](#installation)
2. [Getting Started](#getting-started)
3. [Key Management](#key-management)
4. [Encrypting Secrets](#encrypting-secrets)
5. [Deploying Secrets](#deploying-secrets)
6. [Injecting Secrets into Pods](#injecting-secrets-into-pods)
7. [AllowedSubjects](#allowedsubjects)
8. [Troubleshooting](#troubleshooting)
9. [Best Practices](#best-practices)

## Installation

### CLI Installation

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

### Controller Installation

**Using kubectl:**
```bash
kubectl apply -f config/crd/bases/
kubectl apply -f config/rbac/
kubectl apply -f config/webhook/
```

**Using Helm:**
```bash
helm repo add zen-lock https://kube-zen.github.io/zen-lock
helm repo update
helm install zen-lock zen-lock/zen-lock --namespace zen-lock-system --create-namespace
```

## Configuration

### Environment Variables

zen-lock supports the following environment variables:

- **`ZEN_LOCK_PRIVATE_KEY`** (Required): The private key used to decrypt secrets. Must be set for the controller to function.
- **`ZEN_LOCK_CACHE_TTL`** (Optional): Cache TTL for ZenLock CRDs. Default: `5m` (5 minutes). Format: Go duration string (e.g., `10m`, `1h`).
- **`ZEN_LOCK_ORPHAN_TTL`** (Optional): Time after which orphaned Secrets (Pods not found) are deleted. Default: `15m` (15 minutes). Format: Go duration string.

Example:
```bash
export ZEN_LOCK_PRIVATE_KEY=$(cat private-key.age)
export ZEN_LOCK_CACHE_TTL=10m
export ZEN_LOCK_ORPHAN_TTL=30m
```

## Getting Started

### 1. Generate Encryption Keys

First, generate a key pair for encryption:

```bash
zen-lock keygen --output private-key.age
```

This creates a private key file and displays the public key. **Keep the private key secure** - it's needed to decrypt secrets.

### 2. Extract Public Key

Extract the public key to share with your team:

```bash
zen-lock pubkey --input private-key.age > public-key.age
```

Share `public-key.age` with developers who need to encrypt secrets. **Never share the private key.**

### 3. Configure Controller

The controller needs access to the private key to decrypt secrets. Store it securely:

**Option A: Kubernetes Secret (Recommended)**
```bash
kubectl create secret generic zen-lock-master-key \
  --from-file=key.txt=private-key.age \
  -n zen-lock-system
```

Then update the Deployment to reference it:
```yaml
env:
  - name: ZEN_LOCK_PRIVATE_KEY
    valueFrom:
      secretKeyRef:
        name: zen-lock-master-key
        key: key.txt
```

## Encrypting Secrets

### Create a Secret File

Create a YAML file with your secret data:

```yaml
metadata:
  name: db-credentials
stringData:
  DB_USER: "admin"
  DB_PASS: "SuperSecret123!"
  API_KEY: "sk-1234567890abcdef"
```

### Encrypt the Secret

Encrypt the secret using the public key:

```bash
zen-lock encrypt \
  --pubkey $(cat public-key.age) \
  --input secret.yaml \
  --output encrypted-secret.yaml
```

This creates a `ZenLock` CRD with encrypted data that can be safely committed to Git.

### Example Encrypted Output

```yaml
apiVersion: security.kube-zen.io/v1alpha1
kind: ZenLock
metadata:
  name: db-credentials
  namespace: production
spec:
  encryptedData:
    DB_USER: YWdlLWVuY3J5cHRpb24ub3JnL3YxCi0+IFgyNTUxOSB...
    DB_PASS: YWdlLWVuY3J5cHRpb24ub3JnL3YxCi0+IFgyNTUxOSB...
    API_KEY: YWdlLWVuY3J5cHRpb24ub3JnL3YxCi0+IFgyNTUxOSB...
  algorithm: age
```

## Deploying Secrets

### Apply to Cluster

Deploy the encrypted secret to your cluster:

```bash
kubectl apply -f encrypted-secret.yaml
```

### Verify Deployment

Check the ZenLock status:

```bash
kubectl get zenlock db-credentials -n production
kubectl describe zenlock db-credentials -n production
```

The status should show `Phase: Ready` and `Decryptable: True` if the private key is correctly configured.

## Injecting Secrets into Pods

### Basic Injection

Add annotations to your Pod/Deployment:

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
    spec:
      containers:
      - name: app
        image: nginx
```

The webhook automatically:
1. Decrypts the secret
2. Creates an ephemeral Secret
3. Mounts it to `/zen-secrets` by default

### Custom Mount Path

Specify a custom mount path:

```yaml
annotations:
  zen-lock/inject: "db-credentials"
  zen-lock/mount-path: "/etc/config"
```

### Accessing Secrets in Containers

Secrets are mounted as files in the specified directory:

```bash
# In your container
cat /zen-secrets/DB_USER
cat /zen-secrets/DB_PASS
```

Or use as environment variables (if your app supports it):

```yaml
env:
  - name: DB_USER
    valueFrom:
      secretKeyRef:
        name: zen-lock-inject-<pod-uid>
        key: DB_USER
```

## AllowedSubjects

Restrict which ServiceAccounts can use a secret:

```yaml
apiVersion: security.kube-zen.io/v1alpha1
kind: ZenLock
metadata:
  name: db-credentials
spec:
  encryptedData:
    DB_PASS: <encrypted>
  allowedSubjects:
    - kind: ServiceAccount
      name: backend-app
      namespace: production
```

Only Pods using the `backend-app` ServiceAccount can inject this secret. Other Pods will be denied by the webhook.

### Multiple Allowed Subjects

```yaml
allowedSubjects:
  - kind: ServiceAccount
    name: backend-app
    namespace: production
  - kind: ServiceAccount
    name: worker-app
    namespace: production
```

## Troubleshooting

### Pod Stuck in ContainerCreating

**Check webhook logs:**
```bash
kubectl logs -n zen-lock-system deployment/zen-lock-webhook
```

**Verify private key is set:**
```bash
kubectl get deployment zen-lock-webhook -n zen-lock-system -o yaml | grep ZEN_LOCK_PRIVATE_KEY
```

**Check ZenLock status:**
```bash
kubectl get zenlock <name> -o yaml
kubectl describe zenlock <name>
```

### Webhook Denial

If Pod creation is denied:

1. **Check AllowedSubjects**: Verify the Pod's ServiceAccount is in the allowed list
2. **Check webhook logs**: Look for denial reasons
3. **Verify ZenLock exists**: Ensure the ZenLock CRD exists in the namespace

### Decryption Errors

If decryption fails:

1. **Verify private key**: Ensure `ZEN_LOCK_PRIVATE_KEY` matches the key used for encryption
2. **Check ciphertext**: Verify the encrypted data is valid base64
3. **Check controller logs**: Look for decryption error messages

### Secret Not Mounted

If the secret isn't appearing in the Pod:

1. **Check annotations**: Verify `zen-lock/inject` annotation is present
2. **Check webhook**: Verify webhook is running and processing requests
3. **Check Pod events**: Look for webhook-related events
4. **Verify namespace**: Ensure ZenLock exists in the same namespace as the Pod

## Choosing zen-lock vs Alternatives

zen-lock is designed for **static secrets in GitOps workflows**. Use this decision tree:

**Use zen-lock when**:
- ✅ Static secrets + GitOps is the goal
- ✅ You want encrypted manifests in version control
- ✅ Simple, declarative secret injection is sufficient
- ✅ You prefer Kubernetes-native patterns (CRDs, webhooks)

**Use alternatives when**:
- ❌ You need dynamic secrets or credential rotation (use Vault Agent Injector)
- ❌ You want to avoid Kubernetes Secret objects (use Secrets Store CSI Driver)
- ❌ Centralized policy and audit are required (use Vault)
- ❌ You need integration with external secret providers at runtime (use Vault/CSI/1Password Operator)

**Alternatives**:
- **Vault Agent Injector**: [HashiCorp Vault Agent Injector](https://developer.hashicorp.com/vault/docs/platform/k8s/injector)
- **Secrets Store CSI Driver**: [Secrets Store CSI Driver](https://secrets-store-csi-driver.sigs.k8s.io/)
- **1Password Kubernetes Operator**: [1Password Kubernetes Operator](https://developer.1password.com/docs/connect/kubernetes-operator)

See [FAQ.md](FAQ.md) for detailed positioning and [INTEGRATIONS.md](INTEGRATIONS.md) for integration strategies.

## Best Practices

### Key Management

1. **Store private keys securely**: Use Kubernetes Secrets or external KMS
2. **Rotate keys regularly**: Plan for key rotation (v1.0.0 feature)
3. **Limit access**: Only grant private key access to the controller
4. **Backup keys**: Keep secure backups of private keys

### Secret Organization

1. **Namespace isolation**: Use namespaces to separate secrets
2. **Naming conventions**: Use descriptive names for ZenLocks
3. **AllowedSubjects**: Always use AllowedSubjects for production secrets
4. **GitOps**: Commit encrypted secrets to Git, never plaintext

### Security

1. **Use AllowedSubjects**: Restrict secret access to specific ServiceAccounts
2. **Network policies**: Restrict webhook network access
3. **RBAC**: Limit who can create/modify ZenLocks
4. **Audit logging**: Enable Kubernetes audit logging for ZenLock operations

### Performance

1. **Mount path**: Use appropriate mount paths for your application
2. **Secret size**: Keep secrets reasonably sized
3. **Multiple secrets**: Use separate ZenLocks for different purposes

## Examples

### Complete Example

```yaml
# 1. Encrypt secret
# secret.yaml
metadata:
  name: app-secrets
stringData:
  DATABASE_URL: "postgresql://user:pass@db:5432/mydb"
  REDIS_URL: "redis://redis:6379"

# Encrypt
zen-lock encrypt --pubkey $(cat public-key.age) --input secret.yaml --output app-secrets.yaml

# 2. Deploy ZenLock
kubectl apply -f app-secrets.yaml

# 3. Use in Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  template:
    metadata:
      annotations:
        zen-lock/inject: "app-secrets"
        zen-lock/mount-path: "/etc/secrets"
    spec:
      serviceAccountName: backend-app
      containers:
      - name: app
        image: my-app:latest
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: zen-lock-inject-<pod-uid>
              key: DATABASE_URL
```

## See Also

- [API Reference](API_REFERENCE.md) - Complete API documentation
- [Architecture](ARCHITECTURE.md) - System architecture
- [RBAC](RBAC.md) - RBAC permissions
- [Security Best Practices](SECURITY_BEST_PRACTICES.md) - Security guidelines
- [Testing Guide](TESTING.md) - Testing infrastructure
- [Metrics](METRICS.md) - Prometheus metrics
- [README](../README.md) - Project overview

