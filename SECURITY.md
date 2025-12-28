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

- Secrets are encrypted client-side before being stored in etcd
- The API server cannot read encrypted secrets
- Decryption happens only in the webhook, in-memory
- Decrypted secrets are ephemeral and tied to Pod lifecycle

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

## Security Contact

- **Email**: security@kube-zen.io
- **GitHub**: Use Security tab on repository

Thank you for helping keep zen-lock secure! 🔒

