# RBAC Documentation

This document describes the RBAC (Role-Based Access Control) permissions required for zen-lock.

## Overview

zen-lock requires specific RBAC permissions to:
- Watch and reconcile ZenLock CRDs
- Create ephemeral Secrets for Pods
- Read Pod information for webhook injection
- Record events and manage leader election

## RBAC Architecture

zen-lock uses separate roles for the controller and webhook to follow the principle of least privilege:

- **zen-lock-controller**: Minimal permissions for controller operations
- **zen-lock-webhook**: Minimal permissions for webhook operations
- **zen-lock-manager**: Deprecated combined role (kept for backward compatibility)

## Controller Role

The `zen-lock-controller` ClusterRole includes the following permissions:

### Controller: ZenLock CRD Permissions

```yaml
- apiGroups: ["security.zen.io"]
  resources: ["zenlocks"]
  verbs: ["get", "list", "watch"]
```

**Purpose**: Controller needs read access to:
- Watch for new/updated ZenLocks
- Reconcile ZenLock state

```yaml
- apiGroups: ["security.zen.io"]
  resources: ["zenlocks/status"]
  verbs: ["get", "update", "patch"]
```

**Purpose**: Update ZenLock status fields (Phase, Conditions, etc.)

### Controller: Secret Permissions

```yaml
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list", "watch", "update", "patch"]
```

**Purpose**: Controller needs to:
- Read Secrets created by webhook (to set OwnerReferences)
- Update Secrets to add OwnerReferences when Pod exists
- Watch Secrets for reconciliation

**Note**: Controller does NOT create or delete Secrets - webhook creates them, Kubernetes deletes them via OwnerReference.

### Controller: Pod Permissions

```yaml
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]
```

**Purpose**: Controller needs read access to:
- Get Pod UID for setting OwnerReferences
- Verify Pod exists before setting OwnerReference

## Webhook Role

The `zen-lock-webhook` ClusterRole includes the following permissions:

### Webhook: ZenLock CRD Permissions

```yaml
- apiGroups: ["security.zen.io"]
  resources: ["zenlocks"]
  verbs: ["get"]
```

**Purpose**: Webhook needs read access to:
- Fetch ZenLock CRD during Pod admission
- Decrypt secret data

**Note**: Webhook does NOT need list/watch - it only reads specific ZenLocks by name.

### Webhook: Secret Permissions

```yaml
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["create"]
```

**Purpose**: Webhook needs to:
- Create ephemeral Secrets for Pod injection

**Note**: Webhook does NOT need get/list/watch/update/delete - it only creates Secrets.

### Webhook: Pod Permissions

```yaml
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get"]
```

**Purpose**: Webhook needs read access to:
- Read Pod metadata during admission (for ServiceAccount validation)
- Get Pod name and namespace for Secret naming

**Note**: Webhook does NOT modify Pods directly - it uses mutating admission webhooks which operate on admission requests.

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

## ClusterRoleBindings

### Controller Binding

The `zen-lock-controller` ClusterRoleBinding binds the controller role to the webhook ServiceAccount (since controller and webhook run in the same process):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: zen-lock-controller
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: zen-lock-controller
subjects:
  - kind: ServiceAccount
    name: zen-lock-webhook
    namespace: zen-lock-system
```

**Note**: Both controller and webhook run in the same binary (`zen-lock-webhook`), so they share the same ServiceAccount (`zen-lock-webhook`). Both ClusterRoles are bound to this ServiceAccount.

### Webhook Binding

The `zen-lock-webhook` ClusterRoleBinding binds the webhook role to the webhook ServiceAccount:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: zen-lock-webhook
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: zen-lock-webhook
subjects:
  - kind: ServiceAccount
    name: zen-lock-webhook
    namespace: zen-lock-system
```

## ServiceAccounts

zen-lock uses a single ServiceAccount for both controller and webhook (they run in the same process):

### Webhook ServiceAccount

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: zen-lock-webhook
  namespace: zen-lock-system
```

**Note**: Both the controller (`ZenLockReconciler`, `SecretReconciler`) and webhook (`PodHandler`) run in the same binary (`zen-lock-webhook`), so they share the same ServiceAccount. Both `zen-lock-controller` and `zen-lock-webhook` ClusterRoles are bound to this ServiceAccount.

## Deprecated: Combined Role

The `zen-lock-manager` ClusterRole is deprecated but kept for backward compatibility:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: zen-lock-manager
  annotations:
    rbac.authorization.k8s.io/justification: "DEPRECATED: Use zen-lock-controller and zen-lock-webhook roles instead."
```

**Migration**: Update deployments to use separate roles (`controller-role.yaml` and `webhook-role.yaml`).

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

