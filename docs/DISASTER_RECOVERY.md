# Disaster Recovery

This document describes disaster recovery procedures for zen-lock, including recovery from webhook failures, secret exposure incidents, backup strategies, emergency stop procedures, and rollback scenarios.

## Table of Contents

- [Recovery from Webhook Failures](#recovery-from-webhook-failures)
- [Recovery from Secret Exposure](#recovery-from-secret-exposure)
- [Backup Strategies](#backup-strategies)
- [Emergency Stop Procedures](#emergency-stop-procedures)
- [Rollback Scenarios](#rollback-scenarios)

---

## Recovery from Webhook Failures

### Immediate Response

If you discover that the webhook is failing or Pods cannot be created:

#### Step 1: Check Webhook Status

```bash
# Check webhook configuration
kubectl get mutatingwebhookconfiguration zen-lock-mutating-webhook

# Check webhook service
kubectl get svc zen-lock-webhook -n zen-lock-system
kubectl get endpoints zen-lock-webhook -n zen-lock-system

# Check controller pods
kubectl get pods -n zen-lock-system -l app.kubernetes.io/name=zen-lock

# Check logs
kubectl logs -n zen-lock-system -l app.kubernetes.io/name=zen-lock --tail=100
```

#### Step 2: Temporarily Disable Webhook (Emergency)

If the webhook is blocking all Pod creation:

```bash
# Delete the webhook configuration (emergency only)
kubectl delete mutatingwebhookconfiguration zen-lock-mutating-webhook

# This will allow Pods to be created without secret injection
# WARNING: Secrets will not be injected until webhook is restored
```

#### Step 3: Restore Webhook

After fixing the issue:

```bash
# Restore webhook configuration
kubectl apply -f config/webhook/manifests.yaml

# Or if using Helm
helm upgrade zen-lock zen-lock/zen-lock \
  --namespace zen-lock-system \
  --reuse-values
```

### Common Webhook Issues

#### Certificate Expiration

```bash
# Check certificate status
kubectl get certificate -n zen-lock-system
kubectl describe certificate zen-lock-webhook-cert -n zen-lock-system

# If using cert-manager, check CertificateRequest
kubectl get certificaterequest -n zen-lock-system

# Force certificate renewal (if using cert-manager)
kubectl delete certificate zen-lock-webhook-cert -n zen-lock-system
# cert-manager will automatically recreate it
```

#### Service Endpoint Issues

```bash
# Check service endpoints
kubectl get endpoints zen-lock-webhook -n zen-lock-system

# If endpoints are empty, check pod labels
kubectl get pods -n zen-lock-system -l app.kubernetes.io/name=zen-lock

# Restart pods if needed
kubectl rollout restart deployment/zen-lock-webhook -n zen-lock-system
```

#### Private Key Issues

```bash
# Check if private key is accessible
kubectl get secret zen-lock-master-key -n zen-lock-system

# Check if private key is set in deployment
kubectl get deployment zen-lock-webhook -n zen-lock-system -o yaml | grep ZEN_LOCK_PRIVATE_KEY

# Verify private key format
kubectl exec -n zen-lock-system deployment/zen-lock-webhook -- \
  sh -c 'echo "$ZEN_LOCK_PRIVATE_KEY" | head -1'
# Should start with "AGE-SECRET-KEY-1"
```

---

## Recovery from Secret Exposure

### Immediate Response

If you discover that secrets have been exposed or compromised:

#### Step 1: Assess Exposure

```bash
# Check which ZenLocks are affected
kubectl get zenlocks --all-namespaces

# Check which Pods have secrets mounted
kubectl get pods -A -o json | \
  jq -r '.items[] | select(.spec.volumes[]?.secret?.secretName | startswith("zen-lock-inject")) | "\(.metadata.namespace)/\(.metadata.name)"'

# Check ephemeral secrets
kubectl get secrets -A -l zen-lock.security.kube-zen.io/zenlock-name
```

#### Step 2: Rotate Private Key

**Critical**: If the private key is compromised, rotate it immediately:

```bash
# Generate new key pair
zen-lock keygen --output new-private-key.age

# Extract new public key
zen-lock pubkey --input new-private-key.age > new-public-key.age

# Update controller with new private key
kubectl create secret generic zen-lock-master-key \
  --from-file=key.txt=new-private-key.age \
  -n zen-lock-system \
  --dry-run=client -o yaml | kubectl apply -f -

# Restart controller to pick up new key
kubectl rollout restart deployment/zen-lock-webhook -n zen-lock-system
```

#### Step 3: Re-encrypt All Secrets

All existing ZenLocks must be re-encrypted with the new public key:

```bash
# For each ZenLock, decrypt with old key and re-encrypt with new key
# This is a manual process - automate if you have many ZenLocks

# Example:
zen-lock decrypt --key old-private-key.age --input encrypted-secret.yaml | \
  zen-lock encrypt --pubkey new-public-key.age --output re-encrypted-secret.yaml

# Apply re-encrypted ZenLock
kubectl apply -f re-encrypted-secret.yaml
```

#### Step 4: Revoke Old Key

- Remove old private key from all systems
- Notify team to use new public key
- Update documentation with new public key

### Prevention Measures

1. **Secure Key Storage**: Use external secret management systems
2. **Key Rotation Policy**: Rotate keys regularly (e.g., quarterly)
3. **Access Control**: Limit who has access to private keys
4. **Audit Logging**: Monitor access to private keys
5. **Encryption at Rest**: Ensure etcd encryption is enabled

---

## Backup Strategies

### ZenLock Backup

#### Manual Backup

```bash
# Backup all ZenLocks
kubectl get zenlocks --all-namespaces -o yaml > zenlocks-backup-$(date +%Y%m%d).yaml

# Backup specific namespace
kubectl get zenlocks -n <namespace> -o yaml > zenlocks-<namespace>-backup.yaml
```

#### Automated Backup

Create a CronJob to backup ZenLocks regularly:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: zen-lock-backup
  namespace: zen-lock-system
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: bitnami/kubectl:latest
            command:
            - /bin/sh
            - -c
            - |
              kubectl get zenlocks --all-namespaces -o yaml > /backup/zenlocks-$(date +%Y%m%d).yaml
              # Upload to S3 or other storage
              # aws s3 cp /backup/zenlocks-*.yaml s3://backup-bucket/zen-lock/
          restartPolicy: OnFailure
          volumes:
          - name: backup
            emptyDir: {}
```

### Private Key Backup

**Critical**: Backup private keys securely:

```bash
# Backup private key (store in secure location, e.g., encrypted S3 bucket)
aws s3 cp private-key.age s3://secure-backup-bucket/zen-lock/private-key-$(date +%Y%m%d).age \
  --server-side-encryption aws:kms \
  --sse-kms-key-id <kms-key-id>
```

**Warning**: Never commit private keys to Git or store them unencrypted.

### Backup Retention Policy

- **Daily Backups**: Keep for 7 days
- **Weekly Backups**: Keep for 4 weeks
- **Monthly Backups**: Keep for 12 months
- **Yearly Backups**: Keep indefinitely

---

## Emergency Stop Procedures

### "Break Glass" Emergency Stop

In case of emergency, follow these steps:

#### Method 1: Scale Down Controller

```bash
# Fastest method - stops all webhook operations immediately
kubectl scale deployment zen-lock-webhook -n zen-lock-system --replicas=0
```

#### Method 2: Delete Webhook Configuration

```bash
# More drastic - completely removes webhook
kubectl delete mutatingwebhookconfiguration zen-lock-mutating-webhook
```

**Warning**: This will allow Pods to be created without secret injection. Existing Pods with injected secrets will continue to work, but new Pods will not have secrets injected.

#### Method 3: Delete Deployment

```bash
# Most drastic - completely removes controller
kubectl delete deployment zen-lock-webhook -n zen-lock-system
```

#### Method 4: Network Policy Block

```yaml
# Block all network traffic to webhook
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: zen-lock-block-all
  namespace: zen-lock-system
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: zen-lock
  policyTypes:
  - Ingress
  - Egress
  # No rules = block all traffic
```

### Emergency Stop Script

Create a script for quick emergency stops:

```bash
#!/bin/bash
# emergency-stop.sh

set -e

echo "🚨 EMERGENCY STOP - Stopping zen-lock Webhook"
echo "This will stop all secret injection operations"

# Scale down controller
kubectl scale deployment zen-lock-webhook -n zen-lock-system --replicas=0

# Wait for pods to terminate
kubectl wait --for=delete pod -l app.kubernetes.io/name=zen-lock -n zen-lock-system --timeout=60s

echo "✅ zen-lock Webhook stopped"
echo "⚠️  Remember to investigate and fix issues before restarting"
```

### Restarting After Emergency Stop

1. **Investigate**: Review what caused the emergency stop
2. **Fix Issues**: Correct configuration, certificates, or other issues
3. **Test**: Verify webhook works with a test Pod
4. **Restart**: Scale controller back up

```bash
# Restart controller
kubectl scale deployment zen-lock-webhook -n zen-lock-system --replicas=1

# Verify controller is running
kubectl get pods -n zen-lock-system -l app.kubernetes.io/name=zen-lock

# Test webhook
kubectl run test-pod --image=busybox --restart=Never \
  --overrides='{"metadata":{"annotations":{"zen-lock/inject":"test-zenlock"}}}'
```

---

## Rollback Scenarios

### Rollback Controller Version

#### Method 1: Helm Rollback

```bash
# List releases
helm list -n zen-lock-system

# Rollback to previous version
helm rollback zen-lock -n zen-lock-system

# Rollback to specific revision
helm rollback zen-lock <revision-number> -n zen-lock-system
```

#### Method 2: kubectl Rollout

```bash
# View rollout history
kubectl rollout history deployment/zen-lock-webhook -n zen-lock-system

# Rollback to previous revision
kubectl rollout undo deployment/zen-lock-webhook -n zen-lock-system

# Rollback to specific revision
kubectl rollout undo deployment/zen-lock-webhook -n zen-lock-system --to-revision=<revision-number>
```

### Rollback ZenLock Changes

#### Restore from Backup

```bash
# Restore ZenLocks from backup
kubectl apply -f zenlocks-backup-20251229.yaml

# Or restore specific ZenLock
kubectl apply -f zenlock-<name>-backup.yaml
```

#### Git-based Rollback

If ZenLocks are managed via GitOps:

```bash
# Revert to previous commit
git revert <commit-hash>

# Or checkout previous version
git checkout <previous-commit-hash> -- <zenlock-file>
kubectl apply -f <zenlock-file>
```

### Rollback CRD Version

If CRD schema changes cause issues:

```bash
# List CRD versions
kubectl get crd zenlocks.security.kube-zen.io -o yaml | grep -A 5 "versions:"

# Restore previous CRD version
kubectl apply -f config/crd/bases/security.kube-zen.io_zenlocks.yaml

# Migrate existing resources (if needed)
# This depends on the specific migration path
```

### Rollback Checklist

- [ ] Identify the issue causing rollback
- [ ] Determine rollback target (version/revision)
- [ ] Backup current state
- [ ] Execute rollback procedure
- [ ] Verify rollback success
- [ ] Monitor for issues
- [ ] Document rollback reason and procedure

---

## Disaster Recovery Testing

### Regular Testing Schedule

- **Monthly**: Test emergency stop procedure
- **Quarterly**: Test full disaster recovery
- **Annually**: Test complete cluster recovery

### Test Scenarios

1. **Webhook Failure**: Simulate webhook certificate expiration
2. **Private Key Compromise**: Test key rotation procedure
3. **Secret Exposure**: Test secret rotation and re-encryption
4. **Controller Crash**: Test recovery from controller failure
5. **API Server Failure**: Test behavior during API server issues

### Testing Procedure

```bash
# 1. Create test environment
kubectl create namespace disaster-recovery-test

# 2. Create test ZenLock
kubectl apply -f test-zenlock.yaml -n disaster-recovery-test

# 3. Create test Pod
kubectl run test-pod --image=busybox --restart=Never \
  --overrides='{"metadata":{"annotations":{"zen-lock/inject":"test-zenlock"}}}' \
  -n disaster-recovery-test

# 4. Simulate disaster
# (e.g., delete webhook, corrupt private key, etc.)

# 5. Execute recovery procedure
./emergency-stop.sh
# ... recovery steps ...

# 6. Verify recovery
kubectl get pods -n disaster-recovery-test
kubectl get zenlock -n disaster-recovery-test

# 7. Cleanup
kubectl delete namespace disaster-recovery-test
```

---

## Recovery Time Objectives (RTO) and Recovery Point Objectives (RPO)

### Recommended Targets

- **RTO (Recovery Time Objective)**: < 15 minutes
  - Time to stop webhook failures
  - Time to restore controller functionality
  - Time to rotate compromised keys

- **RPO (Recovery Point Objective)**: < 1 hour
  - Maximum acceptable data loss
  - Backup frequency should support this

### Achieving RTO/RPO

1. **Automated Backups**: Reduce manual intervention
2. **Documented Procedures**: Speed up recovery
3. **Regular Testing**: Ensure procedures work
4. **Monitoring**: Detect issues quickly
5. **Alerting**: Notify team immediately

---

## Summary

Disaster recovery for zen-lock requires:

1. **Prevention**: Secure key storage, regular backups, monitoring
2. **Detection**: Monitor webhook health, secret access, error rates
3. **Response**: Have emergency stop procedures ready
4. **Recovery**: Maintain backups and know how to restore
5. **Testing**: Regularly test recovery procedures

Always have a plan, test it regularly, and keep backups current.

