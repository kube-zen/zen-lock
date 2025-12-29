# zen-lock Helm Chart

A Helm chart for deploying zen-lock, a Zero-Knowledge secret manager for Kubernetes.

## Introduction

zen-lock provides Zero-Knowledge secret management for Kubernetes, ensuring secrets are never stored in plaintext in etcd and never visible via kubectl. Secrets are encrypted at rest and only decrypted in Pod memory.

## Prerequisites

- Kubernetes 1.20+
- Helm 3.0+
- kubectl configured to access your cluster

## Installation

### Quick Start

```bash
# Add the repository
helm repo add zen-lock https://kube-zen.github.io/zen-lock
helm repo update

# Install zen-lock
helm install zen-lock zen-lock/zen-lock \
  --namespace zen-lock-system \
  --create-namespace
```

**Note**: The chart is also available on [Artifact Hub](https://artifacthub.io/packages/helm/zen-lock/zen-lock).

### From Local Chart

```bash
# Install from local chart directory
helm install zen-lock ./charts/zen-lock \
  --namespace zen-lock-system \
  --create-namespace
```

## Configuration

### Private Key Setup

**IMPORTANT**: Before zen-lock can operate, you must provide a private key:

1. Generate a private key:
   ```bash
   zen-lock keygen > master-key.txt
   ```

2. Create the secret:
   ```bash
   kubectl create secret generic zen-lock-master-key \
     --from-file=key.txt=master-key.txt \
     --namespace zen-lock-system
   ```

3. Or update the placeholder secret:
   ```bash
   kubectl create secret generic zen-lock-master-key \
     --from-file=key.txt=master-key.txt \
     --namespace zen-lock-system \
     --dry-run=client -o yaml | kubectl apply -f -
   ```

### Configuration Values

Key configuration options (see `values.yaml` for all options):

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Container image repository | `kubezen/zen-lock` |
| `image.tag` | Container image tag | `0.0.1-alpha` |
| `replicaCount` | Number of replicas | `1` |
| `webhook.enabled` | Enable mutating webhook | `true` |
| `webhook.certManager.enabled` | Use cert-manager for TLS | `false` |
| `privateKey.secretName` | Secret name containing private key | `zen-lock-master-key` |
| `privateKey.secretKey` | Key within secret | `key.txt` |
| `namespace.create` | Create namespace | `true` |
| `namespace.name` | Namespace name | `zen-lock-system` |
| `metrics.enabled` | Enable metrics endpoint | `true` |
| `metrics.port` | Metrics port | `8080` |

### Example: Custom Configuration

```yaml
# custom-values.yaml
image:
  repository: my-registry/zen-lock
  tag: v1.0.0

webhook:
  certManager:
    enabled: true
    issuer:
      name: letsencrypt-prod
      kind: ClusterIssuer

privateKey:
  secretName: my-zen-lock-key
  secretKey: private-key

resources:
  limits:
    cpu: 1000m
    memory: 1Gi
  requests:
    cpu: 200m
    memory: 256Mi
```

Install with custom values:
```bash
helm install zen-lock ./charts/zen-lock \
  --namespace zen-lock-system \
  --create-namespace \
  -f custom-values.yaml
```

## Usage

### Enable zen-lock for a Namespace

Add the label to enable webhook injection:

```bash
kubectl label namespace my-namespace zen-lock=enabled
```

### Create a ZenLock Secret

1. Encrypt your secret:
   ```bash
   echo -n "my-secret-value" | zen-lock encrypt --public-key <public-key> > encrypted.txt
   ```

2. Create ZenLock CRD:
   ```yaml
   apiVersion: security.kube-zen.io/v1alpha1
   kind: ZenLock
   metadata:
     name: my-secret
     namespace: my-namespace
   spec:
     encryptedData:
       password: <encrypted-value-from-encrypted.txt>
   ```

3. Use in Pod:
   ```yaml
   apiVersion: v1
   kind: Pod
   metadata:
     name: my-app
     namespace: my-namespace
     annotations:
       zen-lock/inject: my-secret
   spec:
     containers:
     - name: app
       image: my-app:latest
   ```

## Uninstallation

```bash
helm uninstall zen-lock --namespace zen-lock-system
```

**Note**: This will not delete the CRDs. To remove CRDs:

```bash
kubectl delete crd zenlocks.security.kube-zen.io
```

## Troubleshooting

### Webhook Not Injecting

1. Check namespace label:
   ```bash
   kubectl get namespace my-namespace --show-labels
   ```
   Should show `zen-lock=enabled`

2. Check webhook configuration:
   ```bash
   kubectl get mutatingwebhookconfiguration zen-lock-mutating-webhook
   ```

3. Check webhook logs:
   ```bash
   kubectl logs -n zen-lock-system deployment/zen-lock
   ```

### Private Key Issues

1. Verify secret exists:
   ```bash
   kubectl get secret zen-lock-master-key -n zen-lock-system
   ```

2. Check secret key format:
   ```bash
   kubectl get secret zen-lock-master-key -n zen-lock-system -o jsonpath='{.data.key\.txt}' | base64 -d
   ```
   Should start with `AGE-SECRET-1`

## Security Considerations

- **Private Key**: Store the private key securely. Consider using a secret management system like Vault or AWS Secrets Manager.
- **RBAC**: The chart creates ClusterRole/ClusterRoleBinding with minimal required permissions.
- **TLS**: Use cert-manager for production TLS certificate management.
- **Network Policies**: Consider adding NetworkPolicies to restrict webhook access.

## Chart Repository

- **GitHub Pages**: https://kube-zen.github.io/zen-lock
- **Artifact Hub**: https://artifacthub.io/packages/helm/zen-lock/zen-lock

## Support

For issues and questions:
- GitHub: https://github.com/kube-zen/zen-lock
- Documentation: https://github.com/kube-zen/zen-lock/docs
- Helm Repository: See [docs/HELM_REPOSITORY.md](../../docs/HELM_REPOSITORY.md)

