# Frequently Asked Questions (FAQ)

## What zen-lock is

- **A GitOps-first, Kubernetes-native secret packaging and injection layer.**
- **Source-of-truth is encrypted in a CRD; runtime injection uses short-lived Kubernetes Secrets.**

zen-lock provides a simple, declarative way to manage secrets in Kubernetes using encrypted CRDs that can be safely committed to Git. Secrets are decrypted at admission time and injected into Pods as ephemeral Kubernetes Secrets.

## What zen-lock is not (non-goals)

zen-lock is **not**:

- **A centralized secrets platform**: No authentication methods, policy engine, or audit devices. zen-lock focuses solely on encryption-at-rest for GitOps workflows.
- **Dynamic secrets/leased credentials**: Does not provide database or cloud credential rotation. Secrets are static and encrypted at authoring time.
- **A general "sync secrets from external providers" operator**: zen-lock does not fetch secrets from external systems at runtime. See [INTEGRATIONS.md](INTEGRATIONS.md) for authoring-time integration patterns.
- **A hard security boundary against cluster-admin**: Cluster administrators with Secret read access can view decrypted ephemeral Secrets. RBAC and etcd encryption-at-rest are required for defense-in-depth.

## Where plaintext can exist (threat model clarity)

### ZenLock CRD (Source-of-Truth)
- **Ciphertext only**: The ZenLock CRD stored in etcd contains only unreadable ciphertext.
- **API server cannot decrypt**: The Kubernetes API server cannot read the encrypted data.
- **Safe for Git**: Encrypted manifests can be safely committed to version control.

### Runtime (Ephemeral Secrets)
- **Plaintext in Kubernetes Secrets**: Decrypted secrets are stored as standard Kubernetes Secrets in etcd.
- **RBAC protection**: Access is controlled via Kubernetes RBAC (principals with Secret read access can view plaintext).
- **etcd encryption-at-rest recommended**: Enable etcd encryption-at-rest for additional protection of ephemeral Secrets.
- **Short-lived**: Secrets exist only during Pod lifetime and are automatically cleaned up when Pods terminate.

**Important**: Zero-knowledge applies to the source-of-truth object; runtime delivery necessarily exposes plaintext to the workload and (via Kubernetes Secret) to any principal with Secret read access.

## Is this a Vault replacement?

**No**—zen-lock overlaps with one Kubernetes delivery pattern, but Vault's core value is centralized policy, authentication, audit, and dynamic secrets.

**Use zen-lock when**:
- Static secrets + GitOps is the goal
- You want encrypted manifests in version control
- Simple, declarative secret injection is sufficient

**Use Vault when**:
- You need dynamic secrets (database credentials, cloud IAM roles)
- Centralized policy and audit are required
- Multiple authentication methods are needed
- Secret rotation and leasing are required

See [INTEGRATIONS.md](INTEGRATIONS.md) for integration strategies with Vault and other secret managers.

## Alternatives (when zen-lock is the wrong tool)

### Vault Agent Injector
- **Pattern**: Sidecar rendering secrets to a shared volume
- **Use when**: You need Vault's dynamic secrets, policy engine, or audit capabilities
- **Reference**: [HashiCorp Vault Agent Injector](https://developer.hashicorp.com/vault/docs/platform/k8s/injector)

### Secrets Store CSI Driver
- **Pattern**: Mount external secrets stores as volumes
- **Use when**: You need to avoid Kubernetes Secret objects or integrate with cloud secret managers
- **Reference**: [Secrets Store CSI Driver](https://secrets-store-csi-driver.sigs.k8s.io/)

### 1Password Kubernetes Operator
- **Pattern**: Sync 1Password items into Kubernetes Secrets
- **Use when**: You already use 1Password and want automated secret synchronization
- **Reference**: [1Password Kubernetes Operator](https://developer.1password.com/docs/connect/kubernetes-operator)

### Decision Guidance

**Use zen-lock when**:
- Static secrets + GitOps is the goal
- You want encrypted manifests in version control
- Simple, declarative secret injection is sufficient
- You prefer Kubernetes-native patterns (CRDs, webhooks)

**Use Vault/CSI/etc when**:
- You need dynamic secrets or credential rotation
- You want to avoid Kubernetes Secret objects
- Centralized policy and audit are required
- You need integration with external secret providers at runtime

## See Also

- [User Guide](USER_GUIDE.md) - Complete usage instructions
- [Architecture](ARCHITECTURE.md) - System design and trade-offs
- [Security Best Practices](SECURITY_BEST_PRACTICES.md) - Security guidelines
- [Integrations](INTEGRATIONS.md) - Integration strategies with external systems
- [API Reference](API_REFERENCE.md) - Complete API documentation

