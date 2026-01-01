package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
	"github.com/kube-zen/zen-lock/pkg/controller"
	webhookpkg "github.com/kube-zen/zen-lock/pkg/webhook"
	"github.com/kube-zen/zen-sdk/pkg/leader"
	sdklog 	"github.com/kube-zen/zen-sdk/pkg/logging"
	"github.com/kube-zen/zen-sdk/pkg/zenlead"
)

var (
	scheme                  = runtime.NewScheme()
	logger                  *sdklog.Logger
	setupLog                *sdklog.Logger
	leaderElectionMode      = flag.String("leader-election-mode", "builtin", "Leader election mode: builtin (default), zenlead, or disabled (controller only)")
	leaderElectionID        = flag.String("leader-election-id", "", "The ID for leader election (default: zen-lock-controller-leader-election). Required for builtin mode.")
	leaderElectionLeaseName = flag.String("leader-election-lease-name", "", "The LeaderGroup CRD name (required for zenlead mode)")
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

	flag.Parse()

	// Initialize zen-sdk logger (configures controller-runtime logger automatically)
	logger = sdklog.NewLogger("zen-lock")
	setupLog = logger.WithComponent("setup")

	// OpenTelemetry tracing initialization can be added here when zen-sdk/pkg/observability is available
	// For now, continue without tracing

	// Check for private key
	if os.Getenv("ZEN_LOCK_PRIVATE_KEY") == "" {
		setupLog.Error(fmt.Errorf("ZEN_LOCK_PRIVATE_KEY not set"), "Private key environment variable is required", sdklog.ErrorCode("MISSING_PRIVATE_KEY"))
		os.Exit(1)
	}

	// Build manager options
	baseOpts := ctrl.Options{
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
	// - Controller: Use zenlead package with Profiles B/C
	// - Webhook-only: Explicitly disable leader election (webhooks scale horizontally)
	var mgrOpts ctrl.Options
	if enableController {
		// Get namespace (required for leader election)
		namespace, err := leader.RequirePodNamespace()
		if err != nil {
			setupLog.Error(err, "failed to determine pod namespace (required when controller is enabled)", sdklog.ErrorCode("NAMESPACE_ERROR"))
			os.Exit(1)
		}

		// Apply REST config defaults
		restCfg := ctrl.GetConfigOrDie()
		zenlead.ControllerRuntimeDefaults(restCfg)

		// Configure leader election using zenlead package (Profiles B/C)
		var leConfig zenlead.LeaderElectionConfig

		// Determine election ID (default if not provided)
		electionID := *leaderElectionID
		if electionID == "" {
			electionID = "zen-lock-controller-leader-election"
		}

		// Configure based on mode
		switch *leaderElectionMode {
		case "builtin":
			leConfig = zenlead.LeaderElectionConfig{
				Mode:       zenlead.BuiltIn,
				ElectionID: electionID,
				Namespace:  namespace,
			}
			setupLog.Info("Leader election mode: builtin (Profile B)", sdklog.Operation("leader_election_config"))
		case "zenlead":
			if *leaderElectionLeaseName == "" {
				setupLog.Error(fmt.Errorf("--leader-election-lease-name is required when --leader-election-mode=zenlead"), "invalid configuration", sdklog.ErrorCode("INVALID_CONFIG"))
				os.Exit(1)
			}
			leConfig = zenlead.LeaderElectionConfig{
				Mode:      zenlead.ZenLeadManaged,
				LeaseName: *leaderElectionLeaseName,
				Namespace: namespace,
			}
			setupLog.Info("Leader election mode: zenlead managed (Profile C)", sdklog.Operation("leader_election_config"), sdklog.String("leaseName", *leaderElectionLeaseName))
		case "disabled":
			leConfig = zenlead.LeaderElectionConfig{
				Mode: zenlead.Disabled,
			}
			setupLog.Info("Leader election disabled - single replica only (unsafe if replicas > 1)", sdklog.Operation("leader_election_config"))
		default:
			setupLog.Error(fmt.Errorf("invalid --leader-election-mode: %q (must be builtin, zenlead, or disabled)", *leaderElectionMode), "invalid configuration", sdklog.ErrorCode("INVALID_CONFIG"))
			os.Exit(1)
		}

		// Prepare manager options with leader election
		mgrOpts, err := zenlead.PrepareManagerOptions(&baseOpts, &leConfig)
		if err != nil {
			setupLog.Error(err, "failed to prepare manager options", sdklog.ErrorCode("MANAGER_OPTIONS_ERROR"))
			os.Exit(1)
		}

		// Get replica count from environment (set by Helm/Kubernetes)
		replicaCount := 1
		if rcStr := os.Getenv("REPLICA_COUNT"); rcStr != "" {
			if rc, err := strconv.Atoi(rcStr); err == nil {
				replicaCount = rc
			}
		}

		// Enforce safe HA configuration
		if err := zenlead.EnforceSafeHA(replicaCount, mgrOpts.LeaderElection); err != nil {
			setupLog.Error(err, "unsafe HA configuration", sdklog.ErrorCode("UNSAFE_HA_CONFIG"))
			os.Exit(1)
		}
	} else if enableWebhook {
		// Webhook-only mode: explicitly disable leader election (webhooks scale horizontally)
		mgrOpts = baseOpts
		mgrOpts.LeaderElection = false
		setupLog.Info("Webhook-only mode: leader election disabled (webhooks scale horizontally)", sdklog.Operation("config"))
	} else {
		// Neither enabled - this shouldn't happen, but handle gracefully
		setupLog.Info("Neither controller nor webhook enabled", sdklog.Operation("config"))
		mgrOpts = baseOpts
		mgrOpts.LeaderElection = false
	}

	// Get REST config
	restCfg := ctrl.GetConfigOrDie()
	if enableController {
		// Already applied defaults above
	} else {
		// Apply defaults for webhook-only mode too
		zenlead.ControllerRuntimeDefaults(restCfg)
	}

	mgr, err := ctrl.NewManager(restCfg, mgrOpts)
	if err != nil {
		setupLog.Error(err, "unable to start manager", sdklog.ErrorCode("MANAGER_START_ERROR"))
		os.Exit(1)
	}

	// Setup ZenLock controller (if enabled)
	// Leader election is handled by controller-runtime Manager, not in the reconciler
	if enableController {
		zenlockReconciler, err := controller.NewZenLockReconciler(mgr.GetClient(), mgr.GetScheme())
		if err != nil {
			setupLog.Error(err, "unable to create ZenLock reconciler", sdklog.ErrorCode("RECONCILER_CREATE_ERROR"))
			os.Exit(1)
		}
		if err := zenlockReconciler.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to setup ZenLock controller", sdklog.ErrorCode("CONTROLLER_SETUP_ERROR"))
			os.Exit(1)
		}

		// Setup Secret controller (sets OwnerReferences on webhook-created Secrets)
		secretReconciler := controller.NewSecretReconciler(mgr.GetClient(), mgr.GetScheme())
		if err := secretReconciler.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to setup Secret controller", sdklog.ErrorCode("CONTROLLER_SETUP_ERROR"))
			os.Exit(1)
		}
		setupLog.Info("Controller enabled", sdklog.Component("controller"))
	} else {
		setupLog.Info("Controller disabled", sdklog.Component("controller"))
	}

	// Setup webhook (if enabled)
	if enableWebhook {
		if err := webhookpkg.SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to setup webhook", sdklog.ErrorCode("WEBHOOK_SETUP_ERROR"))
			os.Exit(1)
		}
		setupLog.Info("Webhook enabled", sdklog.Component("webhook"))
	} else {
		setupLog.Info("Webhook disabled", sdklog.Component("webhook"))
	}

	// Ensure at least one component is enabled
	if !enableController && !enableWebhook {
		setupLog.Error(fmt.Errorf("at least one of --enable-controller or --enable-webhook must be true"), "invalid configuration", sdklog.ErrorCode("INVALID_CONFIG"))
		os.Exit(1)
	}

	// Setup health checks
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check", sdklog.ErrorCode("HEALTH_CHECK_ERROR"))
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check", sdklog.ErrorCode("READY_CHECK_ERROR"))
		os.Exit(1)
	}

	setupLog.Info("starting manager", sdklog.Operation("start"))
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager", sdklog.ErrorCode("MANAGER_RUN_ERROR"))
		os.Exit(1)
	}
}
