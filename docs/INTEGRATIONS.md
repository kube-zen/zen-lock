# Integration Strategy

This document outlines zen-lock's integration model with external secret management systems.

## Integration Modes

zen-lock supports two integration modes:

### 1. Authoring-Time Integration

**Pattern**: Pull from provider → encrypt → commit ZenLock CRD (no runtime dependency)

- Secrets are fetched from external systems during development/CI
- Encrypted using zen-lock CLI
- Committed to Git as ZenLock CRDs
- **No runtime dependency** on external systems
- **Benefits**: GitOps-friendly, no availability/latency concerns at admission time

**Example workflow**:
```bash
# Fetch secret from provider
vault kv get -format=json secret/app/db > secret.json

# Encrypt with zen-lock
zen-lock encrypt --pubkey public-key.age --input secret.json --output encrypted-secret.yaml

# Commit to Git
git add encrypted-secret.yaml
git commit -m "Add encrypted database credentials"
```

### 2. Key Custody Integration

**Pattern**: Store zen-lock private key in external system; fetch at startup (policy-driven)

- Private key stored in external secret manager (Vault, 1Password, etc.)
- Controller/webhook fetch key at startup
- Key cached in memory for performance
- **Benefits**: Centralized key management, policy-driven access control

**Example**:
```yaml
env:
  - name: ZEN_LOCK_PRIVATE_KEY
    valueFrom:
      secretKeyRef:
        name: zen-lock-master-key
        key: key.txt
# Secret created by external operator (Vault, 1Password, etc.)
```

### Non-Goal: Runtime Secret Fetching

zen-lock **does not** fetch secrets from external providers during admission time.

**Rationale**:
- Availability/latency blast radius: Webhook failures would block Pod creation
- Complexity: Requires authentication, retry logic, caching
- Alternative: Use Vault Agent Injector or Secrets Store CSI Driver for runtime injection

## Providers

### HashiCorp Vault

**Status**: Planned / Optional

**Integration Points**:

1. **Key Custody**: Store zen-lock private key in Vault
   - Use Vault Agent to inject key as Kubernetes Secret
   - Reference: [Vault Agent](https://developer.hashicorp.com/vault/docs/agent)

2. **Authoring-Time**: Fetch secrets from Vault, encrypt with zen-lock
   - Use Vault CLI or API to fetch secrets
   - Encrypt with zen-lock CLI
   - Commit encrypted ZenLock CRD to Git

**Runtime Injection**: Vault Agent Injector is the canonical runtime injection approach Vault already provides. zen-lock integration focuses on authoring-time and key custody, not duplicating the injector.

**Reference**: [HashiCorp Vault](https://developer.hashicorp.com/vault)

### 1Password

**Status**: Planned / Optional

**Integration Points**:

1. **Key Custody**: Store zen-lock private key in 1Password
   - Use 1Password Kubernetes Operator to sync key as Kubernetes Secret
   - Reference: [1Password Kubernetes Operator](https://developer.1password.com/docs/connect/kubernetes-operator)

2. **Authoring-Time**: Fetch secrets from 1Password, encrypt with zen-lock
   - Use 1Password CLI or API to fetch secrets
   - Encrypt with zen-lock CLI
   - Commit encrypted ZenLock CRD to Git

**Note**: 1Password already provides a Kubernetes Operator and External Secrets Operator (ESO) providers. zen-lock integration is about authoring-time and key custody, not duplicating the operator.

**Reference**: [1Password Developer](https://developer.1password.com/)

### LastPass

**Status**: Best-effort / Community-driven

**Integration Points**:

- Authoring-time: Fetch secrets from LastPass, encrypt with zen-lock
- Key custody: Store zen-lock private key in LastPass (if stable automation API available)

**Note**: Integration depends on LastPass providing a stable automation API. Community contributions welcome.

## Licensing Note (Vault)

HashiCorp Vault is source-available under Business Source License 1.1 (BUSL/BSL) with a defined change license to MPL 2.0 at the change date.

**References**:
- [HashiCorp Vault License](https://github.com/hashicorp/vault/blob/main/LICENSE)
- [HashiCorp Licensing FAQ](https://www.hashicorp.com/license-faq)

**Important**: zen-lock does not embed or redistribute Vault; any integration should be reviewed for compliance in your environment (not legal advice).

## Integration Roadmap

### Phase 1: Authoring-Time Providers (v0.2.0)

- Vault CLI integration helpers
- 1Password CLI integration helpers
- Generic provider abstraction for custom integrations

### Phase 2: Key Custody Providers (v0.2.0)

- Vault key injection via Vault Agent
- 1Password key injection via 1Password Operator
- Generic Kubernetes Secret provider interface

### Phase 3: Optional Enhancements (v1.0.0+)

- CSI driver alignment (avoid Kubernetes Secret objects)
- Direct volume mounting of encrypted data
- **Note**: Not promised unless explicitly committed to roadmap

## See Also

- [FAQ](FAQ.md) - Positioning and limitations
- [User Guide](USER_GUIDE.md) - Usage instructions
- [Security Best Practices](SECURITY_BEST_PRACTICES.md) - Security guidelines
- [Architecture](ARCHITECTURE.md) - System design

