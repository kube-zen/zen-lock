# RBAC Documentation

This document describes the RBAC (Role-Based Access Control) permissions required for zen-lock.

## Overview

zen-lock requires specific RBAC permissions to:
- Watch and reconcile ZenLock CRDs
- Create ephemeral Secrets for Pods
- Read Pod information for webhook injection
- Record events and manage leader election

## ClusterRole

The `zen-lock-manager` ClusterRole includes the following permissions:

### ZenLock CRD Permissions

```yaml
- apiGroups: ["security.zen.io"]
  resources: ["zenlocks"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]
```

**Purpose**: Controller needs full access to ZenLock CRDs to:
- Watch for new/updated ZenLocks
- Update ZenLock status
- Reconcile ZenLock state

```yaml
- apiGroups: ["security.zen.io"]
  resources: ["zenlocks/status"]
  verbs: ["get", "update", "patch"]
```

**Purpose**: Update ZenLock status fields (Phase, Conditions, etc.)

### Secret Permissions

```yaml
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list", "watch", "create"]
```

**Purpose**: Webhook needs to:
- Create ephemeral Secrets for Pods
- Read Secrets to verify creation
- List/Watch for cleanup operations

**Note**: zen-lock does NOT need `delete` permission - Secrets are automatically deleted by Kubernetes when the Pod is deleted (via OwnerReference).

### Pod Permissions

```yaml
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]
```

**Purpose**: Webhook needs read access to:
- Read Pod metadata for injection
- Verify Pod ServiceAccount for AllowedSubjects validation
- Check Pod UID for unique Secret naming

**Note**: zen-lock does NOT modify Pods directly - it uses mutating admission webhooks which operate on admission requests.

### Event Permissions

```yaml
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create"]
```

**Purpose**: Record Kubernetes events for:
- Reconciliation errors
- Webhook injection failures
- Decryption errors

### Lease Permissions

```yaml
- apiGroups: ["coordination.k8s.io"]
  resources: ["leases"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]
```

**Purpose**: Leader election for controller:
- Ensure only one controller instance is active
- Coordinate controller startup/shutdown

## ClusterRoleBinding

The `zen-lock-manager` ClusterRoleBinding binds the ClusterRole to the ServiceAccount:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: zen-lock-manager
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: zen-lock-manager
subjects:
  - kind: ServiceAccount
    name: zen-lock-webhook
    namespace: zen-lock-system
```

## ServiceAccount

The controller runs as a ServiceAccount:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: zen-lock-webhook
  namespace: zen-lock-system
```

## User Permissions

### Creating ZenLocks

Users need permission to create ZenLock CRDs:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: zen-lock-user
rules:
  - apiGroups: ["security.zen.io"]
    resources: ["zenlocks"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

### Reading ZenLocks

For read-only access:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: zen-lock-reader
rules:
  - apiGroups: ["security.zen.io"]
    resources: ["zenlocks"]
    verbs: ["get", "list", "watch"]
```

### Using Secrets in Pods

Users don't need special permissions to use zen-lock secrets in Pods. The webhook handles injection automatically. However, users need:

1. **Permission to create Pods** (standard Kubernetes permission)
2. **ZenLock must exist** in the same namespace as the Pod
3. **AllowedSubjects must allow** the Pod's ServiceAccount (if configured)

## Webhook Permissions

The mutating admission webhook requires:

1. **ValidatingWebhookConfiguration**: Defines which resources the webhook intercepts
2. **MutatingWebhookConfiguration**: Defines webhook endpoint and TLS settings
3. **Certificate**: TLS certificate for secure webhook communication

These are managed by the controller deployment, not RBAC.

## Security Considerations

### Least Privilege

zen-lock follows the principle of least privilege:
- **No delete permissions** on Secrets (handled by OwnerReference)
- **Read-only** access to Pods
- **No access** to other resources

### Namespace Isolation

zen-lock respects namespace boundaries:
- ZenLocks are namespaced resources
- Secrets are created in the same namespace as the Pod
- AllowedSubjects validation is namespace-aware

### Audit Logging

All RBAC operations are logged by Kubernetes audit logging:
- ZenLock CRUD operations
- Secret creation
- Pod read operations
- Event creation

## Troubleshooting RBAC

### Permission Denied Errors

If you see permission denied errors:

1. **Check ClusterRoleBinding**: Verify the ServiceAccount is bound to the ClusterRole
2. **Check ServiceAccount**: Verify the Pod is using the correct ServiceAccount
3. **Check namespace**: Ensure the ServiceAccount exists in the correct namespace
4. **Check logs**: Controller logs will show permission errors

### Webhook Not Working

If the webhook isn't processing requests:

1. **Check ValidatingWebhookConfiguration**: Verify it exists and is configured correctly
2. **Check TLS**: Verify certificates are valid
3. **Check network**: Ensure webhook endpoint is reachable
4. **Check RBAC**: Verify webhook ServiceAccount has required permissions

## Example: Custom RBAC Setup

For custom deployments, you can create your own RBAC:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-zen-lock-sa
  namespace: my-namespace
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: my-zen-lock-role
rules:
  - apiGroups: ["security.zen.io"]
    resources: ["zenlocks"]
    verbs: ["get", "list", "watch", "create", "update", "patch"]
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list", "watch", "create"]
  # ... other rules
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: my-zen-lock-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: my-zen-lock-role
subjects:
  - kind: ServiceAccount
    name: my-zen-lock-sa
    namespace: my-namespace
```

## See Also

- [Security Best Practices](SECURITY_BEST_PRACTICES.md) - Security guidelines
- [Architecture](ARCHITECTURE.md) - System architecture
- [User Guide](USER_GUIDE.md) - User documentation
- [API Reference](API_REFERENCE.md) - Complete API documentation
- [README](../README.md) - Project overview

