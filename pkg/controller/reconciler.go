package controller

import (
	"context"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.zen.io/v1alpha1"
	"github.com/kube-zen/zen-lock/pkg/crypto"
)

// ZenLockReconciler reconciles a ZenLock object
type ZenLockReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	crypto crypto.Encryptor
}

// NewZenLockReconciler creates a new ZenLockReconciler
func NewZenLockReconciler(client client.Client, scheme *runtime.Scheme) (*ZenLockReconciler, error) {
	// Load private key from environment
	privateKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	if privateKey == "" {
		return nil, fmt.Errorf("ZEN_LOCK_PRIVATE_KEY environment variable is not set")
	}

	// Initialize crypto
	encryptor := crypto.NewAgeEncryptor()

	return &ZenLockReconciler{
		Client: client,
		Scheme: scheme,
		crypto: encryptor,
	}, nil
}

//+kubebuilder:rbac:groups=security.zen.io,resources=zenlocks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=security.zen.io,resources=zenlocks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=security.zen.io,resources=zenlocks/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop
func (r *ZenLockReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

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
		return ctrl.Result{}, nil
	}

	// Try to decrypt to verify the secret is valid
	_, err := r.crypto.DecryptMap(zenlock.Spec.EncryptedData, privateKey)
	if err != nil {
		logger.Error(err, "Failed to decrypt ZenLock", "name", zenlock.Name)
		r.updateStatus(ctx, zenlock, "Error", "DecryptionFailed", fmt.Sprintf("Decryption failed: %v", err))
		return ctrl.Result{}, nil
	}

	// Update status to Ready
	r.updateStatus(ctx, zenlock, "Ready", "KeyValid", "Private key loaded and decryption successful")

	return ctrl.Result{}, nil
}

// updateStatus updates the ZenLock status
func (r *ZenLockReconciler) updateStatus(ctx context.Context, zenlock *securityv1alpha1.ZenLock, phase, reason, message string) {
	zenlock.Status.Phase = phase

	// Update or create condition
	condition := securityv1alpha1.ZenLockCondition{
		Type:    "Decryptable",
		Status:  "True",
		Reason:  reason,
		Message: message,
	}

	if phase == "Error" {
		condition.Status = "False"
	}

	// Find existing condition or add new one
	found := false
	for i, c := range zenlock.Status.Conditions {
		if c.Type == condition.Type {
			zenlock.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
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
