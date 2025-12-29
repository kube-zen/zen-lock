package controller

import (
	"context"
	"fmt"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
	"github.com/kube-zen/zen-lock/pkg/controller/metrics"
	"github.com/kube-zen/zen-lock/pkg/crypto"
)

// ZenLockReconciler reconciles a ZenLock object
type ZenLockReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	registry *crypto.Registry
}

// NewZenLockReconciler creates a new ZenLockReconciler
func NewZenLockReconciler(client client.Client, scheme *runtime.Scheme) (*ZenLockReconciler, error) {
	// Load private key from environment
	privateKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	if privateKey == "" {
		return nil, fmt.Errorf("ZEN_LOCK_PRIVATE_KEY environment variable is not set")
	}

	// Initialize crypto registry (supports multiple algorithms)
	registry := crypto.GetGlobalRegistry()

	return &ZenLockReconciler{
		Client:   client,
		Scheme:   scheme,
		registry: registry,
	}, nil
}

//+kubebuilder:rbac:groups=security.kube-zen.io,resources=zenlocks,verbs=get;list;watch
//+kubebuilder:rbac:groups=security.kube-zen.io,resources=zenlocks/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *ZenLockReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	startTime := time.Now()

	// Fetch ZenLock
	zenlock := &securityv1alpha1.ZenLock{}
	if err := r.Get(ctx, req.NamespacedName, zenlock); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Load private key
	privateKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	if privateKey == "" {
		logger.Error(fmt.Errorf("ZEN_LOCK_PRIVATE_KEY not set"), "Cannot decrypt ZenLock")
		r.updateStatus(ctx, zenlock, "Error", "KeyNotFound", "Private key not configured")
		duration := time.Since(startTime).Seconds()
		metrics.RecordReconcile(req.Namespace, req.Name, "error", duration)
		return ctrl.Result{}, nil
	}

	// Get encryptor for the algorithm specified in ZenLock (or default)
	algorithm := zenlock.Spec.Algorithm
	if algorithm == "" {
		algorithm = crypto.GetDefaultAlgorithm()
	}
	encryptor, err := r.registry.Create(algorithm)
	if err != nil {
		logger.Error(err, "Unsupported algorithm", "name", zenlock.Name, "algorithm", algorithm)
		r.updateStatus(ctx, zenlock, "Error", "UnsupportedAlgorithm", fmt.Sprintf("Unsupported algorithm: %s", algorithm))
		duration := time.Since(startTime).Seconds()
		metrics.RecordReconcile(req.Namespace, req.Name, "error", duration)
		return ctrl.Result{}, nil
	}

	// Try to decrypt to verify the secret is valid
	decryptStart := time.Now()
	_, err = encryptor.DecryptMap(zenlock.Spec.EncryptedData, privateKey)
	decryptDuration := time.Since(decryptStart).Seconds()
	if err != nil {
		logger.Error(err, "Failed to decrypt ZenLock", "name", zenlock.Name, "algorithm", algorithm)
		r.updateStatus(ctx, zenlock, "Error", "DecryptionFailed", fmt.Sprintf("Decryption failed: %v", err))
		duration := time.Since(startTime).Seconds()
		metrics.RecordReconcile(req.Namespace, req.Name, "error", duration)
		metrics.RecordDecryption(req.Namespace, req.Name, "error", decryptDuration)
		return ctrl.Result{}, nil
	}

	// Record successful decryption
	metrics.RecordDecryption(req.Namespace, req.Name, "success", decryptDuration)

	// Update status to Ready
	r.updateStatus(ctx, zenlock, "Ready", "KeyValid", "Private key loaded and decryption successful")

	// Record successful reconciliation
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

	if err := r.Status().Update(ctx, zenlock); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update ZenLock status")
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *ZenLockReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.ZenLock{}).
		Complete(r)
}
