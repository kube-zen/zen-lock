# Security Best Practices

This document outlines security best practices for using zen-lock in production environments.

## Table of Contents

1. [Security Model](#security-model)
2. [Key Management](#key-management)
3. [Private Key Storage](#private-key-storage)
4. [Key Rotation](#key-rotation)
5. [Access Control](#access-control)
6. [Network Security](#network-security)
7. [Multi-Tenancy](#multi-tenancy)
8. [Audit and Monitoring](#audit-and-monitoring)
9. [Compliance](#compliance)

## Security Model

### Zero-Knowledge Encryption

zen-lock implements Zero-Knowledge encryption with the following security properties:

**ZenLock CRD (Source of Truth)**:
- Stored in etcd as unreadable ciphertext
- API server cannot decrypt the data
- Safe to commit to Git
- Encrypted client-side before reaching the cluster

**Ephemeral Secrets (Runtime)**:
- Created by webhook as standard Kubernetes Secrets
- Stored in etcd (protected by encryption at rest if configured)
- Contain decrypted plaintext data
- Short-lived (only exist during Pod lifetime)
- Automatically deleted when Pod terminates (via OwnerReference)
- Orphaned Secrets (>1 minute old, Pod not found) are automatically cleaned up
- Stale secrets are validated and refreshed when Pod names are reused

### Important Security Considerations

1. **etcd Encryption at Rest**: While ZenLock CRDs are encrypted, ephemeral Secrets are standard Kubernetes Secrets. Enable etcd encryption at rest for additional protection.

2. **RBAC Controls**: Use RBAC to restrict access to Secrets. The webhook creates Secrets with labels for tracking, and the controller sets OwnerReferences.

3. **Network Policies**: Restrict access to etcd and the webhook endpoint using Network Policies.

4. **Secret Lifetime**: Ephemeral Secrets are automatically cleaned up when Pods terminate, but ensure etcd encryption is enabled for defense-in-depth.

See [Architecture](ARCHITECTURE.md#security-model) for more details.

## Key Management

### Generate Strong Keys

Always use zen-lock's key generation:

```bash
zen-lock keygen --output private-key.age
```

**Never**:
- Use weak or predictable keys
- Reuse keys across environments
- Share private keys via insecure channels

### Key Storage

**Best Practice**: Store private keys in Kubernetes Secrets or external KMS

```bash
# Create Kubernetes Secret
kubectl create secret generic zen-lock-master-key \
  --from-file=key.txt=private-key.age \
  -n zen-lock-system

# Reference in Deployment
env:
  - name: ZEN_LOCK_PRIVATE_KEY
    valueFrom:
      secretKeyRef:
        name: zen-lock-master-key
        key: key.txt
```

**Future**: Use AWS KMS, Google Cloud KMS, or Azure Key Vault (v0.2.0)

### Key Backup

1. **Encrypt backups**: Encrypt private key backups with a separate key
2. **Store securely**: Use secure storage (encrypted at rest)
3. **Limit access**: Only authorized personnel should have backup access
4. **Test restoration**: Regularly test key restoration procedures

### Public Key Distribution

1. **Secure channels**: Distribute public keys via secure channels
2. **Version control**: Store public keys in version control (they're safe to share)
3. **Documentation**: Document which public keys are used for which environments

## Private Key Storage

### Kubernetes Secrets

**Recommended for most deployments:**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: zen-lock-master-key
  namespace: zen-lock-system
type: Opaque
data:
  key.txt: <base64-encoded-private-key>
```

**Security considerations**:
- Enable encryption at rest for etcd
- Use RBAC to restrict Secret access
- Rotate Secrets regularly
- Enable audit logging

### External Secrets Management

**Recommended for production:**

- **AWS KMS**: Store keys in AWS KMS (v0.2.0)
- **Google Cloud KMS**: Use GCP KMS (v0.2.0)
- **Azure Key Vault**: Use Azure Key Vault (v0.2.0)
- **HashiCorp Vault**: Integrate with Vault

### Environment Variables

**Not recommended for production:**

Avoid storing private keys in:
- Plain environment variables
- ConfigMaps
- Unencrypted files
- Version control

## Key Rotation

### Rotation Strategy

1. **Generate new key pair**: Create new private/public key pair
2. **Re-encrypt secrets**: Encrypt all secrets with new public key
3. **Update controller**: Deploy new private key to controller
4. **Gradual rollout**: Update secrets gradually to avoid downtime
5. **Verify**: Ensure all secrets are accessible with new key
6. **Remove old key**: Securely delete old private key

### Automated Rotation

**Future feature (v1.0.0)**: Automated key rotation with zero-downtime

Until then, manual rotation process:

```bash
# 1. Generate new keys
zen-lock keygen --output new-private-key.age

# 2. Re-encrypt all secrets
for secret in secrets/*.yaml; do
  zen-lock encrypt --pubkey $(cat new-public-key.age) --input $secret --output $secret.new
done

# 3. Update controller with new key
kubectl create secret generic zen-lock-master-key-v2 \
  --from-file=key.txt=new-private-key.age \
  -n zen-lock-system

# 4. Update Deployment to use new secret
# 5. Verify all secrets work
# 6. Delete old secret
```

## Access Control

### AllowedSubjects

**Always use AllowedSubjects for production secrets:**

```yaml
apiVersion: security.zen.io/v1alpha1
kind: ZenLock
metadata:
  name: production-secrets
spec:
  encryptedData:
    API_KEY: <encrypted>
  allowedSubjects:
    - kind: ServiceAccount
      name: backend-app
      namespace: production
```

This ensures only authorized ServiceAccounts can access secrets.

### RBAC

**Limit who can create/modify ZenLocks:**

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: zen-lock-admin
rules:
  - apiGroups: ["security.zen.io"]
    resources: ["zenlocks"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

**Grant read-only access to developers:**

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: zen-lock-reader
rules:
  - apiGroups: ["security.zen.io"]
    resources: ["zenlocks"]
    verbs: ["get", "list", "watch"]
```

### Namespace Isolation

Use namespaces to isolate secrets:

- **Development**: `dev` namespace with dev keys
- **Staging**: `staging` namespace with staging keys
- **Production**: `production` namespace with production keys

## Network Security

### Webhook Network Policies

Restrict webhook network access:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: zen-lock-webhook
  namespace: zen-lock-system
spec:
  podSelector:
    matchLabels:
      app: zen-lock-webhook
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: kube-system
    ports:
    - protocol: TCP
      port: 9443
```

### TLS Configuration

Ensure webhook uses TLS:

- **Valid certificates**: Use valid TLS certificates
- **Certificate rotation**: Rotate certificates regularly
- **TLS 1.2+**: Use TLS 1.2 or higher
- **Certificate validation**: Enable certificate validation

## Multi-Tenancy

### Per-Namespace Keys

**Future feature (v0.2.0)**: Per-namespace encryption keys

Until then, use AllowedSubjects to restrict access:

```yaml
allowedSubjects:
  - kind: ServiceAccount
    name: tenant-a-app
    namespace: tenant-a
```

### Cross-Namespace Access

**Avoid cross-namespace secret sharing** unless necessary:

```yaml
# Not recommended
allowedSubjects:
  - kind: ServiceAccount
    name: other-namespace-app
    namespace: other-namespace
```

## Audit and Monitoring

### Enable Audit Logging

Enable Kubernetes audit logging for:

- ZenLock CRUD operations
- Secret creation
- Webhook admission decisions
- RBAC permission checks

### Monitor Metrics

Monitor zen-lock metrics:

- **Reconciliation errors**: `zenlock_reconcile_total{result="error"}`
- **Webhook denials**: `zenlock_webhook_injection_total{result="denied"}`
- **Decryption failures**: `zenlock_decryption_total{result="error"}`

### Alerting

Set up alerts for:

- High error rates
- Webhook injection failures
- Decryption failures
- Controller downtime

## Compliance

### Data Protection

zen-lock helps with compliance by:

- **Encryption at rest (ZenLock CRD)**: Source-of-truth secrets stored as ciphertext in etcd
- **Zero-knowledge (ZenLock CRD)**: API server cannot read encrypted ZenLock data
- **Ephemeral secrets**: Decrypted secrets exist only during Pod lifetime
- **Automatic cleanup**: OwnerReference ensures secrets are deleted when Pods terminate
- **Orphan cleanup**: SecretReconciler automatically deletes orphaned secrets (>1 minute old, Pod not found)
- **Stale-secret prevention**: Webhook validates and refreshes stale secrets when Pod names are reused
- **Audit trail**: All operations are logged
- **RBAC controls**: Fine-grained access control

**Note**: Ephemeral Secrets are standard Kubernetes Secrets. Enable etcd encryption at rest for full protection of decrypted secrets.

### Regulatory Requirements

zen-lock supports compliance with:

- **GDPR**: Encryption and access controls
- **HIPAA**: Secure secret management
- **PCI DSS**: Encryption requirements
- **SOC 2**: Security controls

### Security Scanning

Regularly scan for:

- **Vulnerabilities**: Use `govulncheck` or similar tools
- **Secrets in code**: Use tools like `git-secrets` or `truffleHog`
- **Misconfigurations**: Review RBAC and network policies

## Incident Response

### Key Compromise

If a private key is compromised:

1. **Immediately rotate**: Generate new keys and re-encrypt all secrets
2. **Revoke access**: Remove compromised key from controller
3. **Audit logs**: Review audit logs for unauthorized access
4. **Notify**: Notify affected teams
5. **Document**: Document the incident and remediation steps

### Secret Exposure

If a secret is exposed:

1. **Rotate secret**: Change the exposed secret value
2. **Re-encrypt**: Re-encrypt with new value
3. **Review access**: Review who had access to the secret
4. **Update credentials**: Update any systems using the secret

## See Also

- [User Guide](USER_GUIDE.md) - Usage instructions
- [RBAC](RBAC.md) - RBAC permissions
- [Architecture](ARCHITECTURE.md) - System architecture
- [API Reference](API_REFERENCE.md) - Complete API documentation
- [SECURITY.md](../SECURITY.md) - Security policy
- [README](../README.md) - Project overview

