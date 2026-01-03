package controller

import (
	"context"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
	"github.com/kube-zen/zen-lock/pkg/common"
	"github.com/kube-zen/zen-lock/pkg/config"
	"github.com/kube-zen/zen-lock/pkg/controller/metrics"
	"github.com/kube-zen/zen-lock/pkg/crypto"
	"github.com/kube-zen/zen-lock/pkg/webhook"
	"github.com/kube-zen/zen-sdk/pkg/lifecycle"
	"github.com/kube-zen/zen-sdk/pkg/retry"
)

// ZenLockReconciler reconciles a ZenLock object
type ZenLockReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	crypto     crypto.Encryptor
	privateKey string // Cached private key to avoid repeated env lookups
}

// NewZenLockReconciler creates a new ZenLockReconciler
// Leader election is handled by controller-runtime Manager, not in the reconciler
func NewZenLockReconciler(client client.Client, scheme *runtime.Scheme) (*ZenLockReconciler, error) {
	// Load private key from environment
	privateKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	if privateKey == "" {
		return nil, fmt.Errorf("ZEN_LOCK_PRIVATE_KEY environment variable is not set")
	}

	// Initialize crypto
	encryptor := crypto.NewAgeEncryptor()

	return &ZenLockReconciler{
		Client:     client,
		Scheme:     scheme,
		crypto:     encryptor,
		privateKey: privateKey,
	}, nil
}

//+kubebuilder:rbac:groups=security.kube-zen.io,resources=zenlocks,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=security.kube-zen.io,resources=zenlocks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=security.kube-zen.io,resources=zenlocks/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;delete

const (
	zenLockFinalizer = "zenlocks.security.kube-zen.io/finalizer"
)

// Reconcile is part of the main kubernetes reconciliation loop
func (r *ZenLockReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	startTime := time.Now()

	// Leader election is handled by controller-runtime Manager
	// No need to check leader status here - Manager only starts reconciler on leader

	// Fetch ZenLock
	zenlock := &securityv1alpha1.ZenLock{}
	if err := r.Get(ctx, req.NamespacedName, zenlock); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion
	if lifecycle.IsDeleting(zenlock) {
		return r.handleDeletion(ctx, zenlock, logger, startTime, req)
	}

	// Add finalizer if not present
	if lifecycle.AddFinalizer(zenlock, zenLockFinalizer) {
		if err := r.Update(ctx, zenlock); err != nil {
			logger.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		// Requeue to continue reconciliation (immediate requeue)
		return ctrl.Result{RequeueAfter: 0}, nil
	}

	// Use cached private key, but check if it's still valid (allows for runtime key updates)
	if r.privateKey == "" {
		// Try to reload from environment (allows for key restoration)
		r.privateKey = os.Getenv("ZEN_LOCK_PRIVATE_KEY")
		if r.privateKey == "" {
			logger.Error(fmt.Errorf("ZEN_LOCK_PRIVATE_KEY not set"), "Cannot decrypt ZenLock")
			r.updateStatus(ctx, zenlock, "Error", "KeyNotFound", "Private key not configured")
			duration := time.Since(startTime).Seconds()
			metrics.RecordReconcile(req.Namespace, req.Name, "error", duration)
			// Requeue with delay to allow for key restoration
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}

	// Try to decrypt to verify the secret is valid
	decryptStart := time.Now()
	_, err := r.crypto.DecryptMap(zenlock.Spec.EncryptedData, r.privateKey)
	decryptDuration := time.Since(decryptStart).Seconds()
	if err != nil {
		logger.Error(err, "Failed to decrypt ZenLock", "name", zenlock.Name)
		r.updateStatus(ctx, zenlock, "Error", "DecryptionFailed", fmt.Sprintf("Decryption failed: %v", err))
		duration := time.Since(startTime).Seconds()
		metrics.RecordReconcile(req.Namespace, req.Name, "error", duration)
		metrics.RecordDecryption(req.Namespace, req.Name, "error", decryptDuration)
		return ctrl.Result{}, nil
	}

	// Record successful decryption
	metrics.RecordDecryption(req.Namespace, req.Name, "success", decryptDuration)

	// Invalidate cache when ZenLock is updated (to ensure webhook uses fresh data)
	webhook.InvalidateZenLock(req.NamespacedName)

	// Update status to Ready
	r.updateStatus(ctx, zenlock, "Ready", "KeyValid", "Private key loaded and decryption successful")

	// Record successful reconciliation
	duration := time.Since(startTime).Seconds()
	metrics.RecordReconcile(req.Namespace, req.Name, "success", duration)

	return ctrl.Result{}, nil
}

// handleDeletion handles ZenLock deletion by cleaning up associated Secrets
func (r *ZenLockReconciler) handleDeletion(ctx context.Context, zenlock *securityv1alpha1.ZenLock, logger interface {
	Info(string, ...interface{})
	Error(error, string, ...interface{})
}, startTime time.Time, req ctrl.Request) (ctrl.Result, error) {
	if !lifecycle.HasFinalizer(zenlock, zenLockFinalizer) {
		// Finalizer already removed, nothing to do
		return ctrl.Result{}, nil
	}

	logger.Info("ZenLock is being deleted, cleaning up associated Secrets")

	// List all Secrets with the ZenLock label
	secretList := &corev1.SecretList{}
	if err := r.List(ctx, secretList, client.MatchingLabels{
		common.LabelZenLockName: zenlock.Name,
	}); err != nil {
		logger.Error(err, "Failed to list Secrets for cleanup")
		return ctrl.Result{}, err
	}

	// Delete all associated Secrets
	for _, secret := range secretList.Items {
		if secret.Namespace == zenlock.Namespace {
			if err := r.Delete(ctx, &secret); err != nil {
				logger.Error(err, "Failed to delete Secret", "secret", secret.Name)
				// Continue with other secrets
			} else {
				logger.Info("Deleted Secret", "secret", secret.Name)
			}
		}
	}

	// Remove finalizer
	if err := lifecycle.RemoveFinalizerAndUpdate(ctx, r.Client, zenlock, zenLockFinalizer); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	logger.Info("ZenLock deletion complete")
	duration := time.Since(startTime).Seconds()
	metrics.RecordReconcile(req.Namespace, req.Name, "success", duration)
	return ctrl.Result{}, nil
}

// updateStatus updates the ZenLock status
func (r *ZenLockReconciler) updateStatus(ctx context.Context, zenlock *securityv1alpha1.ZenLock, phase, reason, message string) {
	zenlock.Status.Phase = phase

	now := metav1.Now()
	conditionStatus := "True"
	if phase == "Error" {
		conditionStatus = "False"
	}

	// Update or create condition
	condition := securityv1alpha1.ZenLockCondition{
		Type:    "Decryptable",
		Status:  conditionStatus,
		Reason:  reason,
		Message: message,
	}

	// Find existing condition
	found := false
	var existingIndex int
	for i, c := range zenlock.Status.Conditions {
		if c.Type == condition.Type {
			existingIndex = i
			found = true
			// Only update LastTransitionTime if status changed
			if c.Status != condition.Status {
				condition.LastTransitionTime = &now
			} else {
				condition.LastTransitionTime = c.LastTransitionTime
			}
			break
		}
	}

	if found {
		zenlock.Status.Conditions[existingIndex] = condition
	} else {
		// New condition - set transition time
		condition.LastTransitionTime = &now
		zenlock.Status.Conditions = append(zenlock.Status.Conditions, condition)
	}

	// Retry status update with exponential backoff for transient errors
	retryConfig := retry.DefaultConfig()
	retryConfig.MaxAttempts = config.DefaultRetryMaxAttempts
	retryConfig.InitialDelay = config.DefaultRetryInitialDelay
	retryConfig.MaxDelay = config.DefaultRetryMaxDelay

	if err := retry.Do(ctx, retryConfig, func() error {
		return r.Status().Update(ctx, zenlock)
	}); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update ZenLock status after retries")
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *ZenLockReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.ZenLock{}).
		Complete(r)
}
