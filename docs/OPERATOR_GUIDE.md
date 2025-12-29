# Operator Guide

This guide is for operators who need to install, configure, and maintain zen-lock in Kubernetes clusters.

**Production Readiness**: zen-lock is production-ready for operational concerns (high availability, leader election, comprehensive observability, reliability). Some security features are planned for future versions (KMS integration, automated key rotation, multi-tenancy). See [ROADMAP.md](../ROADMAP.md) for details.

## Table of Contents

- [Installation](#installation)
- [Configuration](#configuration)
- [Monitoring](#monitoring)
- [Troubleshooting](#troubleshooting)
- [Upgrading](#upgrading)
- [Security](#security)

---

## Installation

### Prerequisites

- Kubernetes cluster 1.23+
- kubectl configured
- Cluster admin permissions (for CRD and webhook installation)
- cert-manager (optional, for automatic certificate management)

### Step 1: Install CRD

```bash
kubectl apply -f config/crd/bases/security.kube-zen.io_zenlocks.yaml
```

Verify installation:

```bash
kubectl get crd zenlocks.security.kube-zen.io
```

### Step 2: Prepare Private Key

The controller requires a private key to decrypt secrets. Store it securely:

```bash
# Create a Kubernetes Secret with the private key
kubectl create secret generic zen-lock-master-key \
  --from-file=key.txt=private-key.age \
  -n zen-lock-system
```

**Security Note**: Ensure the private key is stored securely. Consider using:
- External secret management systems (AWS Secrets Manager, HashiCorp Vault, etc.)
- Kubernetes Secrets with encryption at rest enabled
- Key rotation policies

### Step 3: Install Controller and Webhook

**Using Helm (Recommended):**

```bash
helm repo add zen-lock https://kube-zen.github.io/zen-lock
helm repo update
helm install zen-lock zen-lock/zen-lock \
  --namespace zen-lock-system \
  --create-namespace \
  --set privateKey.secretName=zen-lock-master-key \
  --set privateKey.secretKey=key.txt \
  --set privateKey.createPlaceholder=false
```

**Using kubectl:**

```bash
# Create namespace
kubectl create namespace zen-lock-system

# Install RBAC
kubectl apply -f config/rbac/controller-role.yaml
kubectl apply -f config/rbac/webhook-role.yaml

# Install Deployment
kubectl apply -f config/webhook/manifests.yaml

# Configure private key in deployment
kubectl set env deployment/zen-lock-webhook \
  ZEN_LOCK_PRIVATE_KEY="$(cat private-key.age)" \
  -n zen-lock-system
```

### Step 4: Verify Installation

```bash
# Check controller is running
kubectl get pods -n zen-lock-system

# Check webhook configuration
kubectl get mutatingwebhookconfiguration zen-lock-mutating-webhook

# Check logs
kubectl logs -n zen-lock-system -l app.kubernetes.io/name=zen-lock

# Check metrics endpoint
kubectl port-forward -n zen-lock-system svc/zen-lock-metrics 8080:8080
curl http://localhost:8080/metrics
```

---

## Configuration

### Environment Variables

The controller supports the following environment variables:

- **`ZEN_LOCK_PRIVATE_KEY`** (Required): The private key used to decrypt secrets. Must be set for the controller to function.
- **`ZEN_LOCK_CACHE_TTL`** (Optional): Cache TTL for ZenLock CRDs. Default: `5m` (5 minutes). Format: Go duration string (e.g., `10m`, `1h`).
- **`ZEN_LOCK_ORPHAN_TTL`** (Optional): Time after which orphaned Secrets (Pods not found) are deleted. Default: `15m` (15 minutes). Format: Go duration string.

### Webhook Configuration

The webhook can be configured via Helm values:

```yaml
webhook:
  sideEffects: NoneOnDryRun  # Webhook side effects policy
  timeoutSeconds: 10          # Webhook timeout
  failurePolicy: Fail         # Failure policy (Fail or Ignore)
```

### Resource Limits

#### Default Resource Configuration

Default resource limits in deployment:

```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

#### Resource Requirements by Scale

**Small Scale (< 100 ZenLocks, < 1,000 Pods)**
- **CPU Request**: 100m
- **CPU Limit**: 500m
- **Memory Request**: 128Mi
- **Memory Limit**: 512Mi
- **Expected Usage**: ~5-15m CPU, ~45-65MB memory

**Medium Scale (100-500 ZenLocks, 1,000-10,000 Pods)**
- **CPU Request**: 200m
- **CPU Limit**: 1000m (1 CPU)
- **Memory Request**: 256Mi
- **Memory Limit**: 1Gi
- **Expected Usage**: ~15-50m CPU, ~65-120MB memory

**Large Scale (500-1,000 ZenLocks, 10,000-50,000 Pods)**
- **CPU Request**: 500m
- **CPU Limit**: 2000m (2 CPU)
- **Memory Request**: 512Mi
- **Memory Limit**: 2Gi
- **Expected Usage**: ~50-150m CPU, ~120-300MB memory

**Very Large Scale (> 1,000 ZenLocks, > 50,000 Pods)**
- **CPU Request**: 1000m (1 CPU)
- **CPU Limit**: 4000m (4 CPU)
- **Memory Request**: 1Gi
- **Memory Limit**: 4Gi
- **Expected Usage**: ~150-400m CPU, ~300-800MB memory

#### Adjusting Resources

Adjust resources based on:
- **Number of ZenLocks**: More ZenLocks = more memory for cache
- **Number of Pods**: More Pods = more webhook requests
- **Webhook request rate**: Higher rates = more CPU
- **Cache TTL**: Longer TTL = more memory

#### Monitoring Resource Usage

Monitor these metrics to determine if resources need adjustment:

```bash
# Check current resource usage
kubectl top pods -n zen-lock-system -l app.kubernetes.io/name=zen-lock

# Monitor memory usage via metrics
curl http://localhost:8080/metrics | grep go_memstats

# Check for OOMKills
kubectl describe pod -n zen-lock-system -l app.kubernetes.io/name=zen-lock | grep -i oom
```

**Signs that resources need adjustment:**
- CPU throttling (check `container_cpu_cfs_throttled_seconds_total`)
- Memory pressure (check `container_memory_working_set_bytes`)
- OOMKills (pod restarts)
- Slow webhook responses (check `zenlock_webhook_injection_duration_seconds`)
- High error rates (check `zenlock_webhook_injection_total{result="error"}`)

---

## Monitoring

### Metrics

The controller exposes Prometheus metrics on port 8080:

```bash
# Port forward to access metrics
kubectl port-forward -n zen-lock-system svc/zen-lock-metrics 8080:8080

# View metrics
curl http://localhost:8080/metrics
```

Key metrics to monitor:
- `zenlock_reconcile_total` - Reconciliation rate
- `zenlock_webhook_injection_total` - Webhook injection rate
- `zenlock_decryption_total` - Decryption operations
- `zenlock_webhook_injection_duration_seconds` - Webhook latency
- `zenlock_cache_hits_total` - Cache performance
- `zenlock_webhook_validation_failures_total` - Validation failures

See [Metrics Documentation](METRICS.md) for complete metrics reference.

### Health Checks

- `/healthz` - Liveness probe
- `/readyz` - Readiness probe

### Logging

Controller logs include:
- ZenLock reconciliation events
- Webhook injection events
- Secret creation/cleanup events
- Errors and warnings

View logs:

```bash
# View all pods
kubectl logs -n zen-lock-system -l app.kubernetes.io/name=zen-lock -f

# View specific pod
kubectl logs -n zen-lock-system deployment/zen-lock-webhook -f
```

### Events

The controller emits Kubernetes events for:
- ZenLock lifecycle (created, updated, deleted)
- Reconciliation results
- Webhook injection results
- Errors

View events:

```bash
# View all zen-lock events
kubectl get events -n zen-lock-system --field-selector involvedObject.kind=ZenLock

# View events for specific ZenLock
kubectl describe zenlock <name> -n <namespace>
```

---

## Troubleshooting

### Controller Not Starting

1. **Check CRD installation:**
   ```bash
   kubectl get crd zenlocks.security.kube-zen.io
   ```

2. **Check RBAC:**
   ```bash
   kubectl get clusterrole zen-lock-controller
   kubectl get clusterrole zen-lock-webhook
   kubectl get clusterrolebinding zen-lock-controller
   kubectl get clusterrolebinding zen-lock-webhook
   ```

3. **Check private key:**
   ```bash
   kubectl get secret zen-lock-master-key -n zen-lock-system
   kubectl logs -n zen-lock-system -l app.kubernetes.io/name=zen-lock | grep -i "private key"
   ```

4. **Check pod status:**
   ```bash
   kubectl describe pod -n zen-lock-system -l app.kubernetes.io/name=zen-lock
   ```

### Webhook Not Working

1. **Check webhook configuration:**
   ```bash
   kubectl get mutatingwebhookconfiguration zen-lock-mutating-webhook -o yaml
   ```

2. **Check webhook service:**
   ```bash
   kubectl get svc zen-lock-webhook -n zen-lock-system
   kubectl get endpoints zen-lock-webhook -n zen-lock-system
   ```

3. **Check certificates:**
   ```bash
   kubectl get certificate -n zen-lock-system
   kubectl describe certificate zen-lock-webhook-cert -n zen-lock-system
   ```

4. **Test webhook manually:**
   ```bash
   # Create a test Pod with annotations
   kubectl run test-pod --image=busybox --restart=Never \
     --overrides='{"metadata":{"annotations":{"zen-lock/inject":"test-zenlock"}}}'
   
   # Check if secret was created
   kubectl get secret | grep zen-lock
   ```

### Secrets Not Being Injected

1. **Check Pod annotations:**
   ```bash
   kubectl get pod <pod-name> -o yaml | grep -A 5 annotations
   ```

2. **Check namespace label:**
   ```bash
   kubectl get namespace <namespace> -o yaml | grep zen-lock
   # Webhook only processes namespaces with label: zen-lock=enabled
   kubectl label namespace <namespace> zen-lock=enabled
   ```

3. **Check ZenLock exists:**
   ```bash
   kubectl get zenlock <name> -n <namespace>
   kubectl describe zenlock <name> -n <namespace>
   ```

4. **Check AllowedSubjects:**
   ```bash
   kubectl get zenlock <name> -n <namespace> -o yaml | grep -A 10 allowedSubjects
   # Ensure Pod's ServiceAccount matches AllowedSubjects
   ```

5. **Check webhook logs:**
   ```bash
   kubectl logs -n zen-lock-system -l app.kubernetes.io/name=zen-lock | grep -i "inject\|denied\|error"
   ```

### High Resource Usage

1. **Reduce cache TTL** (if memory is high)
2. **Optimize ZenLock count** - Consolidate similar secrets
3. **Increase resource limits** if needed
4. **Monitor cache hit rate** - Low hit rate may indicate cache TTL too short

### Secret Cleanup Issues

1. **Check orphan TTL:**
   ```bash
   kubectl get deployment zen-lock-webhook -n zen-lock-system -o yaml | grep ZEN_LOCK_ORPHAN_TTL
   ```

2. **Check Secret labels:**
   ```bash
   kubectl get secret -A -l zen-lock.security.kube-zen.io/zenlock-name
   ```

3. **Check OwnerReferences:**
   ```bash
   kubectl get secret <secret-name> -o yaml | grep -A 5 ownerReferences
   ```

---

## Upgrading

### Backup ZenLocks

Before upgrading, backup existing ZenLocks:

```bash
kubectl get zenlocks --all-namespaces -o yaml > zenlocks-backup-$(date +%Y%m%d).yaml
```

### Upgrade Steps

1. **Backup ZenLocks** (see above)

2. **Update CRD** (if changed):
   ```bash
   kubectl apply -f config/crd/bases/security.kube-zen.io_zenlocks.yaml
   ```

3. **Update controller** (Helm):
   ```bash
   helm repo update zen-lock
   helm upgrade zen-lock zen-lock/zen-lock \
     --namespace zen-lock-system \
     --reuse-values
   ```

4. **Update controller** (kubectl):
   ```bash
   kubectl set image deployment/zen-lock-webhook \
     zen-lock-webhook=kube-zen/zen-lock:<new-version> \
     -n zen-lock-system
   ```

5. **Verify upgrade:**
   ```bash
   kubectl rollout status deployment/zen-lock-webhook -n zen-lock-system
   kubectl logs -n zen-lock-system -l app.kubernetes.io/name=zen-lock
   ```

### Rollback

If upgrade fails:

```bash
# Helm rollback
helm rollback zen-lock -n zen-lock-system

# kubectl rollback
kubectl rollout undo deployment/zen-lock-webhook -n zen-lock-system
```

---

## Security

### RBAC

zen-lock uses separate ServiceAccounts for controller and webhook:

**Controller ServiceAccount** (`zen-lock-controller`):
- **Read/Update** access to `ZenLock` CRDs and status
- **Create/Get/Update/Delete** access to `Secret` resources
- **Get/List/Watch** access to `Pod` resources
- **Create** access to `Event` resources
- **Full** access to `Lease` resources (for leader election)

**Webhook ServiceAccount** (`zen-lock-webhook`):
- **Get** access to `ZenLock` CRDs (read only)
- **Create/Get/Update** access to `Secret` resources

See [RBAC Documentation](RBAC.md) for complete RBAC details.

### Service Accounts

Both controller and webhook run as non-root users with minimal, component-specific permissions. Each component uses its own ServiceAccount to achieve least privilege.

### Network Policies

If using network policies, allow:
- Controller → API server (all ports)
- Webhook → API server (all ports)
- API server → Webhook service (port 443)
- Prometheus → Controller metrics (port 8080)

### Private Key Management

**Critical**: The private key must be stored securely:

1. **Use Kubernetes Secrets with encryption at rest**
2. **Consider external secret management** (AWS Secrets Manager, HashiCorp Vault, etc.)
3. **Implement key rotation policies**
4. **Limit access to the private key**
5. **Monitor access to the private key**

### Webhook Security

- Webhook uses TLS certificates (managed by cert-manager or manually)
- Webhook validates Pod annotations and mount paths
- Webhook enforces AllowedSubjects restrictions
- Webhook sanitizes error messages to prevent information leakage

---

## Backup and Recovery

### Backup ZenLocks

```bash
# Backup all ZenLocks
kubectl get zenlocks --all-namespaces -o yaml > zenlocks-backup-$(date +%Y%m%d).yaml

# Backup specific namespace
kubectl get zenlocks -n <namespace> -o yaml > zenlocks-<namespace>-backup.yaml
```

### Restore ZenLocks

```bash
kubectl apply -f zenlocks-backup-$(date +%Y%m%d).yaml
```

**Note**: Ephemeral Secrets are not backed up as they are automatically recreated when Pods are created.

---

## Uninstallation

### Remove ZenLocks

```bash
kubectl delete zenlocks --all-namespaces --all
```

### Remove Controller

```bash
# Helm uninstall
helm uninstall zen-lock -n zen-lock-system

# kubectl uninstall
kubectl delete -f config/webhook/manifests.yaml
kubectl delete -f config/rbac/
```

### Remove CRD

```bash
kubectl delete crd zenlocks.security.kube-zen.io
```

**Warning:** Removing the CRD will delete all ZenLocks!

---

## See Also

- [User Guide](USER_GUIDE.md) - How to use zen-lock
- [Metrics Documentation](METRICS.md) - Monitoring and metrics
- [API Reference](API_REFERENCE.md) - Complete API documentation
- [Security Best Practices](SECURITY_BEST_PRACTICES.md) - Security guidelines

