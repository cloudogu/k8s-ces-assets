package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/cloudogu/maintenance-assets/config"
	maintenance2 "github.com/cloudogu/maintenance-assets/controller"
	"github.com/cloudogu/maintenance-assets/logging"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	scheme               = runtime.NewScheme()
	logger               = ctrl.Log.WithName("k8s-ces.assets.maintenance.main")
	metricsAddr          string
	enableLeaderElection bool
	probeAddr            string
)

type k8sManager interface {
	manager.Manager
}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme

	if err := logging.ConfigureLogger(); err != nil {
		logger.Error(err, "unable configure logger")
		os.Exit(1)
	}
}

func main() {
	if err := startManager(); err != nil {
		logger.Error(err, "manager produced an error")
		os.Exit(1)
	}
}

func startManager() error {
	logger.Info("Starting k8s-ces-assets maintenance discovery...")

	watchNamespace, err := config.ReadWatchNamespace()
	if err != nil {
		return fmt.Errorf("failed to read watch namespace: %w", err)
	}

	options := getK8sManagerOptions(watchNamespace)

	serviceDiscManager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		return fmt.Errorf("failed to create new manager: %w", err)
	}

	globalConfigRepo := repository.NewMaintenanceModeAdapter("k8s-ces-assets", serviceDiscManager.GetClient(), watchNamespace)

	if err := handleErrorPageCreation(serviceDiscManager, globalConfigRepo); err != nil {
		return fmt.Errorf("failed to create error-page creator: %w", err)
	}

	if err = startK8sManager(serviceDiscManager); err != nil {
		return fmt.Errorf("failure at error-page manager: %w", err)
	}
	return nil
}

func startK8sManager(k8sManager k8sManager) error {
	logger.Info("starting service discovery manager")

	err := k8sManager.Start(ctrl.SetupSignalHandler())
	if err != nil {
		return fmt.Errorf("failed to start service discovery manager: %w", err)
	}

	return nil
}

func getK8sManagerOptions(watchNamespace string) manager.Options {
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8082", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8083", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	return ctrl.Options{
		Scheme:  scheme,
		Metrics: server.Options{BindAddress: metricsAddr},
		Cache: cache.Options{DefaultNamespaces: map[string]cache.Config{
			watchNamespace: {},
		}},
		WebhookServer:          webhook.NewServer(webhook.Options{Port: 9443}),
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "92a787f2.cloudogu.com",
	}
}

func handleErrorPageCreation(k8sManager k8sManager, maintenanceAdapter maintenance2.MaintenanceAdapter) error {
	maintenanceReconciler := &maintenance2.MaintenanceReconciler{
		Adapter: maintenanceAdapter,
	}

	if err := maintenanceReconciler.SetupWithManager(k8sManager); err != nil {
		return fmt.Errorf("failed to setup maintenance reconciler with the manager: %w", err)
	}

	return nil
}
