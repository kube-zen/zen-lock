package main

import (
	"flag"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
	"github.com/kube-zen/zen-lock/pkg/controller"
	webhookpkg "github.com/kube-zen/zen-lock/pkg/webhook"
	"github.com/kube-zen/zen-sdk/pkg/leader"
)

var (
	scheme              = runtime.NewScheme()
	setupLog            = ctrl.Log.WithName("setup")
	leaderElectionID    = flag.String("leader-election-id", "zen-lock-controller-leader-election", "The ID for leader election. Must be unique per controller instance in the same namespace.")
	enableLeaderElection = flag.Bool("enable-leader-election", true, "Enable leader election for controller HA (default: true). Set to false if you don't want HA or want zen-lead to handle HA instead.")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(securityv1alpha1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var probeAddr string
	var certDir string
	var enableController bool
	var enableWebhook bool

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&certDir, "cert-dir", "/tmp/k8s-webhook-server/serving-certs",
		"The directory where cert-manager injects the TLS certificates.")
	flag.BoolVar(&enableController, "enable-controller", true,
		"Enable the controller (ZenLock and Secret reconcilers). Leader election is mandatory when enabled.")
	flag.BoolVar(&enableWebhook, "enable-webhook", true,
		"Enable the mutating admission webhook. Leader election is disabled for webhook-only mode.")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Check for private key
	if os.Getenv("ZEN_LOCK_PRIVATE_KEY") == "" {
		setupLog.Error(fmt.Errorf("ZEN_LOCK_PRIVATE_KEY not set"), "Private key environment variable is required")
		os.Exit(1)
	}

	// Get namespace for leader election (required if controller is enabled)
	var namespace string
	if enableController {
		var err error
		namespace, err = leader.RequirePodNamespace()
		if err != nil {
			setupLog.Error(err, "failed to determine pod namespace for leader election (required when controller is enabled)")
			os.Exit(1)
		}
	} else {
		// Webhook-only mode doesn't need namespace (no leader election)
		namespace = "zen-lock-system" // Default, not used
	}

	// Build manager options
	mgrOpts := ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    9443,
			CertDir: certDir,
		}),
		HealthProbeBindAddress: probeAddr,
	}

	// Configure leader election based on component type:
	// - Controller: Enable leader election by default (can be disabled via --enable-leader-election=false)
	// - Webhook-only: Never enable leader election (webhooks scale horizontally)
	if enableController {
		leader.ApplyLeaderElection(&mgrOpts, "zen-lock-controller", namespace, *leaderElectionID, *enableLeaderElection)
		if *enableLeaderElection {
			setupLog.Info("Leader election enabled for controller HA")
		} else {
			setupLog.Warning("Leader election disabled - running without HA (split-brain risk if multiple replicas)")
		}
	} else if enableWebhook {
		// Webhook-only mode: no leader election (webhooks scale horizontally)
		mgrOpts.LeaderElection = false
		setupLog.Info("Webhook-only mode: leader election disabled (webhooks scale horizontally)")
	} else {
		// Neither enabled - this shouldn't happen, but handle gracefully
		setupLog.Info("Neither controller nor webhook enabled")
		mgrOpts.LeaderElection = false
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), mgrOpts)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Setup ZenLock controller (if enabled)
	// Leader election is handled by controller-runtime Manager, not in the reconciler
	if enableController {
		zenlockReconciler, err := controller.NewZenLockReconciler(mgr.GetClient(), mgr.GetScheme())
		if err != nil {
			setupLog.Error(err, "unable to create ZenLock reconciler")
			os.Exit(1)
		}
		if err := zenlockReconciler.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to setup ZenLock controller")
			os.Exit(1)
		}

		// Setup Secret controller (sets OwnerReferences on webhook-created Secrets)
		secretReconciler := controller.NewSecretReconciler(mgr.GetClient(), mgr.GetScheme())
		if err := secretReconciler.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to setup Secret controller")
			os.Exit(1)
		}
		setupLog.Info("Controller enabled")
	} else {
		setupLog.Info("Controller disabled")
	}

	// Setup webhook (if enabled)
	if enableWebhook {
		if err := webhookpkg.SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to setup webhook")
			os.Exit(1)
		}
		setupLog.Info("Webhook enabled")
	} else {
		setupLog.Info("Webhook disabled")
	}

	// Ensure at least one component is enabled
	if !enableController && !enableWebhook {
		setupLog.Error(fmt.Errorf("at least one of --enable-controller or --enable-webhook must be true"), "invalid configuration")
		os.Exit(1)
	}

	// Setup health checks
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
