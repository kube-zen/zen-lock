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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kube-zen/zen-lock/pkg/common"
	"github.com/kube-zen/zen-lock/pkg/config"
	"github.com/kube-zen/zen-sdk/pkg/retry"
)

const (
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
	podName, hasPodName := secret.Labels[common.LabelPodName]
	podNamespace, hasPodNamespace := secret.Labels[common.LabelPodNamespace]
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
			return ctrl.Result{RequeueAfter: config.RequeueDelayPodNotFound}, nil
		}
		logger.Error(err, "Failed to get Pod for zen-lock secret", "secret", req.NamespacedName, "pod", podKey)
		return ctrl.Result{}, fmt.Errorf("failed to get Pod: %w", err)
	}

	// Pod exists and has UID - set OwnerReference
	if pod.UID == "" {
		logger.V(4).Info("Pod exists but has no UID yet, will retry", "pod", podKey)
		return ctrl.Result{RequeueAfter: config.RequeueDelayPodNoUID}, nil
	}

	// Set owner reference using zen-sdk/pkg/k8s/metadata
	// This ensures proper scheme handling and garbage collection
	retryConfig := retry.DefaultConfig()
	retryConfig.MaxAttempts = config.DefaultRetryMaxAttempts
	retryConfig.InitialDelay = config.DefaultRetryInitialDelay
	retryConfig.MaxDelay = config.DefaultRetryMaxDelay

	if err := retry.Do(ctx, retryConfig, func() error {
		// Re-fetch secret to get latest version (for conflict resolution)
		currentSecret := &corev1.Secret{}
		if err := r.Get(ctx, req.NamespacedName, currentSecret); err != nil {
			return err
		}
		// Set owner reference using controllerutil
		// This ensures proper scheme handling and garbage collection
		if err := controllerutil.SetOwnerReference(pod, currentSecret, r.Scheme); err != nil {
			return fmt.Errorf("failed to set owner reference: %w", err)
		}
		return r.Update(ctx, currentSecret)
	}); err != nil {
		logger.Error(err, "Failed to update Secret with OwnerReference after retries", "secret", req.NamespacedName, "pod", podKey)
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
