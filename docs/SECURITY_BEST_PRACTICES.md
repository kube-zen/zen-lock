# Security Best Practices

This document outlines security best practices for using zen-lock in production environments.

**Note on Production Readiness**: zen-lock is production-ready for operational concerns (HA, leader election, observability). Some security features are planned for future versions:
- **v0.2.0**: KMS integration (AWS KMS, GCP KMS, Azure Key Vault), multi-tenancy (per-namespace keys)
- **v1.0.0**: Automated key rotation, secret versioning, audit logging integration

See [ROADMAP.md](../ROADMAP.md) for complete feature plans.

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

**Zero-knowledge applies to the source-of-truth object; runtime delivery necessarily exposes plaintext to the workload and (via Kubernetes Secret) to any principal with Secret read access.**

**ZenLock CRD (Source-of-Truth Ciphertext)**:
- Stored in etcd as unreadable ciphertext
- API server cannot decrypt the data
- Safe to commit to Git
- Encrypted client-side before reaching the cluster

**Ephemeral Kubernetes Secret (Runtime Plaintext)**:
- Created by injection webhook (admission-time mutation) as standard Kubernetes Secrets
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

zen-lock integrates with external secret managers for key custody and authoring-time workflows. See [INTEGRATIONS.md](INTEGRATIONS.md) for detailed integration strategies.

**Key Custody** (v0.2.0):
- **AWS KMS**: Store zen-lock private key in AWS KMS
- **Google Cloud KMS**: Store zen-lock private key in GCP KMS
- **Azure Key Vault**: Store zen-lock private key in Azure Key Vault
- **HashiCorp Vault**: Store zen-lock private key in Vault (use Vault Agent for injection)

**Authoring-Time Integration**:
- Fetch secrets from external systems during development/CI
- Encrypt with zen-lock CLI
- Commit encrypted ZenLock CRD to Git
- No runtime dependency on external systems

**Runtime Injection Alternatives**:
- **Vault Agent Injector**: For dynamic secrets and centralized policy
  - Reference: [HashiCorp Vault Agent Injector](https://developer.hashicorp.com/vault/docs/platform/k8s/injector)
  - Use when: You need Vault's dynamic secrets, policy engine, or audit capabilities

- **1Password Kubernetes Operator**: For automated secret synchronization
  - Reference: [1Password Kubernetes Operator](https://developer.1password.com/docs/connect/kubernetes-operator)
  - Use when: You already use 1Password and want automated secret synchronization

**Vault Licensing Note**:

HashiCorp Vault is source-available under Business Source License 1.1 (BUSL/BSL) with a defined change license to MPL 2.0 at the change date.

**References**:
- [HashiCorp Vault License](https://github.com/hashicorp/vault/blob/main/LICENSE)
- [HashiCorp Licensing FAQ](https://www.hashicorp.com/license-faq)

**Important**: zen-lock does not embed or redistribute Vault; any integration should be reviewed for compliance in your environment (not legal advice).

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
apiVersion: security.kube-zen.io/v1alpha1
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
  - apiGroups: ["security.kube-zen.io"]
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
  - apiGroups: ["security.kube-zen.io"]
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

## Known Security Gaps / Trade-offs

This section explicitly documents security trade-offs and limitations of zen-lock. See also [SECURITY.md](../SECURITY.md#known-security-gaps--trade-offs) for the canonical section.

### Plaintext in etcd (Ephemeral Secrets)

**Risk**: The injected ephemeral Kubernetes Secret is a normal Kubernetes Secret (plaintext in etcd unless encryption-at-rest is enabled).

**Mitigation**: Enable etcd encryption-at-rest for defense-in-depth. This is a recommended but not mandatory control.

**Impact**: Cluster administrators or anyone with etcd access can read decrypted secrets if etcd encryption-at-rest is not enabled.

### RBAC Exposure

**Risk**: Any principal with `get`/`list`/`watch` secrets in the namespace can read decrypted data from ephemeral Kubernetes Secrets.

**Mitigation**: Use least-privilege RBAC. Restrict Secret read access to only necessary principals. Use AllowedSubjects to limit which workloads can access which secrets.

**Impact**: Overly permissive RBAC can expose secrets to unauthorized principals.

### Cluster-Admin Reality

**Risk**: Cluster administrators (or anyone with etcd access) can access secrets; zen-lock is not a boundary against that threat model.

**Mitigation**: This is a fundamental limitation of Kubernetes. Use separate clusters, network policies, and audit logging to detect unauthorized access.

**Impact**: zen-lock cannot protect against cluster-admin or etcd access. This is expected behavior for Kubernetes-native secret management.

### Webhook Key Custody Risk

**Risk**: Webhook/controller holds the private key; compromise enables decrypting all ZenLocks.

**Mitigation**: 
- Store private key in external KMS (v0.2.0)
- Use separate ServiceAccounts with least-privilege RBAC
- Enable audit logging
- Rotate keys regularly

**Impact**: Compromise of the webhook/controller ServiceAccount allows decryption of all ZenLocks in the cluster.

### Lifecycle Gaps

**Risk**: Ephemeral Kubernetes Secrets exist without OwnerReferences until the controller reconciles; orphan cleanup is best-effort and TTL-based.

**Mitigation**:
- Controller sets OwnerReference once Pod UID is available
- Orphan cleanup runs with configurable TTL (default: 15 minutes)
- Monitor orphan cleanup metrics

**Impact**: Brief window where Secrets exist without OwnerReference. Orphaned Secrets may persist if controller is unavailable.

## See Also

- [User Guide](USER_GUIDE.md) - Usage instructions
- [RBAC](RBAC.md) - RBAC permissions
- [Architecture](ARCHITECTURE.md) - System architecture
- [API Reference](API_REFERENCE.md) - Complete API documentation
- [SECURITY.md](../SECURITY.md) - Security policy and known gaps
- [FAQ](FAQ.md) - Positioning and limitations
- [README](../README.md) - Project overview

