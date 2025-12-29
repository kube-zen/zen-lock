# Security Policy

## Supported Versions

We release patches to fix security issues. Which versions are eligible for receiving such patches depends on the CVSS v3.0 Rating:

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |
| < 0.1   | :x:                |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report them via one of the following methods:

1. **Email**: security@kube-zen.io (preferred)
2. **GitHub Security Advisory**: Use the "Report a vulnerability" button on the repository's Security tab

### What to Include

When reporting a vulnerability, please include:

- Type of vulnerability
- Full paths of source file(s) related to the vulnerability
- Location of the affected code (tag/branch/commit or direct URL)
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the issue, including how an attacker might exploit it

### Response Timeline

- **Initial Response**: Within 48 hours
- **Status Update**: Within 7 days
- **Fix Timeline**: Depends on severity and complexity
  - **Critical**: As soon as possible (typically < 7 days)
  - **High**: Within 30 days
  - **Medium**: Within 90 days
  - **Low**: Best effort

### Disclosure Policy

- We will acknowledge receipt of your vulnerability report within 48 hours
- We will provide an estimated timeline for a fix
- We will notify you when the vulnerability is fixed
- We will credit you in the security advisory (if desired)

## Security Best Practices

### For Users

1. **Keep Updated**: Always use the latest stable version
2. **Private Key Security**: Store private keys securely (K8s Secrets, KMS, etc.)
3. **RBAC**: Use minimal RBAC permissions
4. **Network Policies**: Restrict network access to webhook
5. **Audit Logs**: Enable Kubernetes audit logging
6. **AllowedSubjects**: Use AllowedSubjects to restrict secret access
7. **Key Rotation**: Rotate encryption keys regularly

### For Developers

1. **Dependencies**: Keep dependencies up to date
2. **Security Scanning**: Run `govulncheck` and `gosec` regularly
3. **Input Validation**: Validate all inputs
4. **Error Handling**: Don't expose sensitive information in errors
5. **Least Privilege**: Use minimal RBAC permissions
6. **Key Management**: Never commit private keys to version control

## Security Checklist

Before deploying:

- [ ] Private key stored securely (K8s Secret or KMS)
- [ ] RBAC permissions reviewed and minimized
- [ ] Security context configured (non-root, read-only filesystem)
- [ ] Network policies applied
- [ ] Webhook TLS certificates properly configured
- [ ] AllowedSubjects configured (if using multi-tenancy)
- [ ] Dependencies scanned for vulnerabilities
- [ ] Audit logging enabled

## Security Considerations

### Zero-Knowledge Architecture

- **ZenLock CRD (Source-of-Truth)**: Secrets are encrypted client-side before being stored in etcd as ciphertext
- **API server cannot decrypt**: The Kubernetes API server cannot read the encrypted ZenLock CRD data
- **Runtime Decryption**: Decryption happens in the webhook process; decrypted data is persisted as a Kubernetes Secret for Pod consumption
- **Ephemeral Secrets**: Decrypted secrets are ephemeral and tied to Pod lifecycle via OwnerReference

**Important**: Zero-knowledge applies to the source-of-truth object; runtime delivery necessarily exposes plaintext to the workload and (via Kubernetes Secret) to any principal with Secret read access.

### Threat Model

**Cluster-Admin Access**:
- Cluster administrators with Secret read access can view decrypted ephemeral Secrets
- RBAC and etcd encryption-at-rest are required for defense-in-depth
- zen-lock is not a hard security boundary against cluster-admin

**Compromised Workload**:
- A compromised workload can read its own injected secrets (mounted as files or environment variables)
- This is expected behavior; secrets are decrypted for workload consumption
- Use AllowedSubjects to limit which workloads can access which secrets

**Webhook Availability**:
- Webhook failures can block Pod creation (failurePolicy: Fail)
- Webhook timeouts can delay Pod startup
- Monitor webhook health and configure appropriate failurePolicy for your use case

### Key Management

- Private keys should be stored in Kubernetes Secrets or external KMS
- Never commit private keys to version control
- Rotate keys regularly
- Use separate keys for different environments

### Webhook Security

- Webhook must use TLS (cert-manager recommended)
- Webhook should be in a dedicated namespace
- Network policies should restrict access to webhook
- Webhook should validate AllowedSubjects

## Known Security Gaps / Trade-offs

This section explicitly documents security trade-offs and limitations of zen-lock.

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

## Security Contact

- **Email**: security@kube-zen.io
- **GitHub**: Use Security tab on repository

Thank you for helping keep zen-lock secure! 🔒

