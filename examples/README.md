# zen-lock Examples

This directory contains example files demonstrating how to use zen-lock.

## Quick Start Example

### 1. Generate Keys

```bash
zen-lock keygen --output private-key.age
# Save the displayed public key to public-key.age
zen-lock pubkey --input private-key.age > public-key.age
```

### 2. Encrypt a Secret

```bash
zen-lock encrypt \
  --pubkey $(cat public-key.age) \
  --input examples/secret.yaml \
  --output encrypted-secret.yaml
```

### 3. Deploy ZenLock CRD

```bash
kubectl apply -f encrypted-secret.yaml
```

### 4. Create Secret in Cluster (for webhook)

```bash
# Create the master key secret that the webhook will use
kubectl create secret generic zen-lock-master-key \
  --from-file=key.txt=private-key.age \
  -n zen-lock-system
```

### 5. Deploy Application

```bash
kubectl apply -f examples/deployment.yaml
```

### 6. Verify Secret Injection

```bash
# Get the pod name
POD_NAME=$(kubectl get pods -l app=my-app -o jsonpath='{.items[0].metadata.name}')

# Check the secret is mounted
kubectl exec $POD_NAME -- ls -la /etc/config

# View a secret value (be careful!)
kubectl exec $POD_NAME -- cat /etc/config/DB_USER
```

## Notes

- The `zen-lock/inject` annotation tells the webhook which ZenLock to inject
- The `zen-lock/mount-path` annotation is optional (defaults to `/zen-secrets`)
- The volume name must be `zen-secrets` (fixed name used by the webhook)
- Secrets are automatically cleaned up when the Pod is deleted (via OwnerReference)

