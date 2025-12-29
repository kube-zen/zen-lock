package controller

import (
	"context"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// Label keys for Secret tracking
	labelPodName      = "zen-lock.security.zen.io/pod-name"
	labelPodNamespace = "zen-lock.security.zen.io/pod-namespace"
	labelZenLockName  = "zen-lock.security.zen.io/zenlock-name"

	// DefaultOrphanTTL is the default time after which an orphaned Secret (Pod not found) is deleted
	// This is configurable via environment variable ZEN_LOCK_ORPHAN_TTL (duration string, e.g., "10m")
	// Default: 15 minutes (safer for slow control planes and high-latency clusters)
	DefaultOrphanTTL = 15 * time.Minute
)

// SecretReconciler reconciles Secrets created by the webhook to set OwnerReferences
type SecretReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	OrphanTTL time.Duration // Time after which orphaned Secrets are deleted
}

// NewSecretReconciler creates a new SecretReconciler
func NewSecretReconciler(client client.Client, scheme *runtime.Scheme) *SecretReconciler {
	orphanTTL := DefaultOrphanTTL
	if ttlStr := os.Getenv("ZEN_LOCK_ORPHAN_TTL"); ttlStr != "" {
		if parsedTTL, err := time.ParseDuration(ttlStr); err == nil {
			orphanTTL = parsedTTL
		}
	}
	return &SecretReconciler{
		Client:    client,
		Scheme:    scheme,
		OrphanTTL: orphanTTL,
	}
}

//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

// Reconcile sets OwnerReference on zen-lock Secrets when the Pod exists
func (r *SecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch Secret
	secret := &corev1.Secret{}
	if err := r.Get(ctx, req.NamespacedName, secret); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Only process Secrets with zen-lock labels
	podName, hasPodName := secret.Labels[labelPodName]
	podNamespace, hasPodNamespace := secret.Labels[labelPodNamespace]
	if !hasPodName || !hasPodNamespace {
		// Not a zen-lock Secret, ignore
		return ctrl.Result{}, nil
	}

	// Check if OwnerReference already exists
	if len(secret.OwnerReferences) > 0 {
		// Already has OwnerReference, nothing to do
		return ctrl.Result{}, nil
	}

	// Fetch the Pod
	pod := &corev1.Pod{}
	podKey := types.NamespacedName{
		Name:      podName,
		Namespace: podNamespace,
	}
	if err := r.Get(ctx, podKey, pod); err != nil {
		// Pod doesn't exist - check if this is a stale/orphaned secret
		if k8serrors.IsNotFound(err) {
			// Check if Secret has been orphaned for a while (no OwnerReference means Pod was never created or was deleted)
			// If Secret is old enough (older than OrphanTTL), it's likely orphaned
			secretAge := time.Since(secret.CreationTimestamp.Time)
			if secretAge > r.OrphanTTL {
				// Secret is orphaned - delete it
				logger.Info("Deleting orphaned zen-lock secret (Pod not found)", "secret", req.NamespacedName, "pod", podKey, "age", secretAge)
				if err := r.Delete(ctx, secret); err != nil {
					logger.Error(err, "Failed to delete orphaned zen-lock secret", "secret", req.NamespacedName)
					return ctrl.Result{}, fmt.Errorf("failed to delete orphaned secret: %w", err)
				}
				return ctrl.Result{}, nil
			}
			// Secret is new, Pod might be created soon - retry
			logger.V(4).Info("Pod not found for Secret, will retry", "pod", podKey, "secret", req.NamespacedName, "age", secretAge)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		logger.Error(err, "Failed to get Pod for zen-lock secret", "secret", req.NamespacedName, "pod", podKey)
		return ctrl.Result{}, fmt.Errorf("failed to get Pod: %w", err)
	}

	// Pod exists and has UID - set OwnerReference
	if pod.UID == "" {
		logger.V(4).Info("Pod exists but has no UID yet, will retry", "pod", podKey)
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
	}

	// Create OwnerReference
	controller := true
	ownerRef := metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "Pod",
		Name:       pod.Name,
		UID:        pod.UID,
		Controller: &controller,
	}

	// Update Secret with OwnerReference
	secret.OwnerReferences = []metav1.OwnerReference{ownerRef}
	if err := r.Update(ctx, secret); err != nil {
		logger.Error(err, "Failed to update Secret with OwnerReference", "secret", req.NamespacedName, "pod", podKey)
		return ctrl.Result{}, fmt.Errorf("failed to update Secret: %w", err)
	}

	logger.Info("Set OwnerReference on Secret", "secret", req.NamespacedName, "pod", podKey)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		Complete(r)
}

