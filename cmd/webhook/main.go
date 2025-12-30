package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(securityv1alpha1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var certDir string
	var enableController bool
	var enableWebhook bool

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for HA (uses zen-lead when enabled). "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&certDir, "cert-dir", "/tmp/k8s-webhook-server/serving-certs",
		"The directory where cert-manager injects the TLS certificates.")
	flag.BoolVar(&enableController, "enable-controller", true,
		"Enable the controller (ZenLock and Secret reconcilers).")
	flag.BoolVar(&enableWebhook, "enable-webhook", true,
		"Enable the mutating admission webhook.")

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

	// Get namespace for leader election
	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		// Try to read from service account namespace file
		if ns, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
			namespace = string(ns)
		} else {
			namespace = "zen-lock-system"
		}
	}

	// Configure leader election: when enabled, use zen-lead (recommended)
	var externalWatcher *leader.Watcher
	var shouldReconcile func() bool = func() bool { return true }

	if enableLeaderElection {
		// Use zen-lead for leader election (recommended)
		shouldReconcile = func() bool {
			if externalWatcher == nil {
				return false // Not initialized yet
			}
			return externalWatcher.GetIsLeader()
		}
		setupLog.Info("Starting with zen-lead leader election. Waiting for leader role...")
	} else {
		// HA disabled - always reconcile (accept split-brain risk)
		shouldReconcile = func() bool { return true }
		setupLog.Info("Running with HA disabled. Accepting split-brain risk. Not recommended for production.")
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

	// Disable built-in leader election (use zen-lead instead when enabled)
	mgrOpts.LeaderElection = false

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), mgrOpts)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Setup external watcher for zen-lead (must be done after manager is created)
	ctx, cancel := signal.NotifyContext(ctrl.SetupSignalHandler(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if enableLeaderElection {
		watcher, err := leader.NewWatcher(mgr.GetClient(), func(isLeader bool) {
			if isLeader {
				setupLog.Info("Elected as leader via zen-lead. Starting reconciliation...")
			} else {
				setupLog.Info("Lost leadership via zen-lead. Pausing reconciliation...")
			}
		})
		if err != nil {
			setupLog.Error(err, "unable to create external leader watcher")
			os.Exit(1)
		}
		externalWatcher = watcher

		// Start watching in background
		go func() {
			if err := watcher.Watch(ctx); err != nil && err != context.Canceled {
				setupLog.Error(err, "error watching leader status")
			}
		}()
	}

	// Setup ZenLock controller (if enabled)
	if enableController {
		var zenlockReconciler *controller.ZenLockReconciler
		if enableLeaderElection {
			zenlockReconciler, err = controller.NewZenLockReconcilerWithLeaderCheck(mgr.GetClient(), mgr.GetScheme(), shouldReconcile)
		} else {
			zenlockReconciler, err = controller.NewZenLockReconciler(mgr.GetClient(), mgr.GetScheme())
		}
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
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
