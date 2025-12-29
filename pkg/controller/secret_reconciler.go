package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
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
)

// SecretReconciler reconciles Secrets created by the webhook to set OwnerReferences
type SecretReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// NewSecretReconciler creates a new SecretReconciler
func NewSecretReconciler(client client.Client, scheme *runtime.Scheme) *SecretReconciler {
	return &SecretReconciler{
		Client: client,
		Scheme: scheme,
	}
}

//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;update;patch
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
		// Pod doesn't exist yet or was deleted - this is fine, we'll retry
		logger.V(4).Info("Pod not found for Secret, will retry", "pod", podKey, "secret", req.NamespacedName)
		return ctrl.Result{RequeueAfter: 5}, nil
	}

	// Pod exists and has UID - set OwnerReference
	if pod.UID == "" {
		logger.V(4).Info("Pod exists but has no UID yet, will retry", "pod", podKey)
		return ctrl.Result{RequeueAfter: 2}, nil
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

