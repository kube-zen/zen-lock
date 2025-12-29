# API Reference

## ZenLock CRD

### Group: `security.kube-zen.io`
### Version: `v1alpha1`
### Kind: `ZenLock`

### Spec

```yaml
apiVersion: security.kube-zen.io/v1alpha1
kind: ZenLock
metadata:
  name: example-secret
  namespace: production
spec:
  # Required: Map of key -> Base64-encoded ciphertext
  encryptedData:
    USERNAME: <base64-encoded-ciphertext>
    API_KEY: <base64-encoded-ciphertext>
  
  # Optional: Encryption algorithm (default: "age")
  algorithm: age
  
  # Optional: List of ServiceAccounts allowed to use this secret
  # Currently only ServiceAccount kind is supported
  allowedSubjects:
  - kind: ServiceAccount
    name: backend-app
    namespace: production
```

### Status

```yaml
status:
  # Phase: Ready or Error
  phase: Ready
  
  # Last key rotation timestamp
  lastRotation: "2015-12-28T00:00:00Z"
  
  # Conditions
  conditions:
  - type: Decryptable
    status: "True"
    reason: "KeyValid"
    message: "Private key loaded and decryption successful"
    lastTransitionTime: "2015-12-28T00:00:00Z"
```

## Annotations

### Pod Annotations

#### `zen-lock/inject`
**Required**: Name of the ZenLock CRD to inject

```yaml
annotations:
  zen-lock/inject: "db-credentials"
```

#### `zen-lock/mount-path`
**Optional**: Custom mount path for secrets (default: `/zen-secrets`)

```yaml
annotations:
  zen-lock/mount-path: "/etc/config"
```

## SubjectReference

```yaml
kind: ServiceAccount  # ServiceAccount, User, or Group
name: backend-app
namespace: production  # Required for ServiceAccount
```

## CLI Commands

### `zen-lock keygen`
Generate a new encryption key pair.

```bash
zen-lock keygen --output private-key.age
```

### `zen-lock pubkey`
Extract public key from private key.

```bash
zen-lock pubkey --input private-key.age > public-key.age
```

### `zen-lock encrypt`
Encrypt a YAML file containing secret data.

```bash
zen-lock encrypt \
  --pubkey age1q3... \
  --input secret.yaml \
  --output encrypted-zenlock.yaml
```

### `zen-lock decrypt`
Decrypt a ZenLock CRD file (debug only).

```bash
zen-lock decrypt \
  --privkey private-key.age \
  --input encrypted-zenlock.yaml \
  --output plain-secret.yaml
```

## See Also

- [User Guide](USER_GUIDE.md) - Complete usage guide
- [Architecture](ARCHITECTURE.md) - System architecture
- [RBAC](RBAC.md) - RBAC permissions
- [Security Best Practices](SECURITY_BEST_PRACTICES.md) - Security guidelines

