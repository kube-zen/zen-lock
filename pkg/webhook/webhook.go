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

	// Register webhook
	mgr.GetWebhookServer().Register("/mutate-pods", &admission.Webhook{
		Handler: podHandler,
	})

	return nil
}

