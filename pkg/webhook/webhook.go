package webhook

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SetupWebhookWithManager sets up the webhook with the manager
func SetupWebhookWithManager(mgr manager.Manager) error {
	// Create pod handler
	podHandler, err := NewPodHandler(mgr.GetClient(), mgr.GetScheme())
	if err != nil {
		return err
	}

	// Create ZenLock validator handler
	zenlockValidatorHandler, err := NewZenLockValidatorHandler(mgr.GetScheme())
	if err != nil {
		return err
	}

	// Register mutating webhook for Pods
	mgr.GetWebhookServer().Register("/mutate-pods", &admission.Webhook{
		Handler: podHandler,
	})

	// Register validating webhook for ZenLocks
	mgr.GetWebhookServer().Register("/validate-zenlock", &admission.Webhook{
		Handler: zenlockValidatorHandler,
	})

	// Note: Rate limiting for admission webhooks is handled at the Kubernetes API server level
	// via timeoutSeconds and failurePolicy. The rate limiting infrastructure is available
	// in pkg/webhook/ratelimit.go for future use if HTTP-level rate limiting is needed.

	return nil
}
