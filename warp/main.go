package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/cloudogu/k8s-registry-lib/dogu"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/cloudogu/warp-assets/config"
	warpCtrl "github.com/cloudogu/warp-assets/controller"
	"github.com/cloudogu/warp-assets/logging"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	doguv2 "github.com/cloudogu/k8s-dogu-lib/v2/api/v2"
)

var (
	scheme               = runtime.NewScheme()
	logger               = ctrl.Log.WithName("k8s-ces.assets.warp.main")
	metricsAddr          string
	enableLeaderElection bool
	probeAddr            string
)

type k8sManager interface {
	manager.Manager
}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(doguv2.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme

	if err := logging.ConfigureLogger(); err != nil {
		logger.Error(err, "unable configure logger")
		os.Exit(1)
	}
}

func main() {
	if err := start(); err != nil {
		logger.Error(err, "manager produced an error")
		os.Exit(1)
	}
}

func start() error {
	logger.Info("Starting k8s-ces-assets warp discovery...")

	watchNamespace, err := config.ReadWatchNamespace()
	if err != nil {
		return fmt.Errorf("read config value 'watch namespace': %w", err)
	}

	options := getK8sManagerOptions(watchNamespace)

	warpMenuManager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		return fmt.Errorf("create manager: %w", err)
	}

	if err = setupWarpMenuReconciler(warpMenuManager, watchNamespace); err != nil {
		return fmt.Errorf("setup up reconciler: %w", err)
	}

	if err = startManager(warpMenuManager); err != nil {
		return fmt.Errorf("start manager: %w", err)
	}
	return nil
}

func setupWarpMenuReconciler(warpMenuManager k8sManager, watchNamespace string) error {
	clientset, err := getK8sClientSet(warpMenuManager.GetConfig())
	if err != nil {
		return fmt.Errorf("create k8s client set: %w", err)
	}

	client := warpMenuManager.GetClient()
	configMapInterface := clientset.CoreV1().ConfigMaps(watchNamespace)
	globalConfigRepo := repository.NewGlobalConfigRepository(configMapInterface)
	doguVersionRegistry := dogu.NewDoguVersionRegistry(configMapInterface)
	localDoguRepo := dogu.NewLocalDoguDescriptorRepository(configMapInterface)

	deploymentName, err := config.ReadDeploymentName()
	if err != nil {
		return fmt.Errorf("read config value 'deployment name': %w", err)
	}
	eventRecorder := warpMenuManager.GetEventRecorderFor(deploymentName)

	warpMenuPath, err := config.ReadWarpPath()
	if err != nil {
		return fmt.Errorf("read config value 'warp path': %w", err)
	}
	reconciler := warpCtrl.NewWarpMenuReconciler(client, globalConfigRepo, doguVersionRegistry, localDoguRepo, eventRecorder, warpMenuPath, deploymentName)
	err = reconciler.SetupWithManager(warpMenuManager)
	if err != nil {
		return fmt.Errorf("setup reconciler with manager: %w", err)
	}

	return nil
}

func startManager(k8sManager k8sManager) error {
	logger.Info("starting manager")

	err := k8sManager.Start(ctrl.SetupSignalHandler())
	if err != nil {
		return fmt.Errorf("start manager: %w", err)
	}

	return nil
}

func getK8sClientSet(config *rest.Config) (*kubernetes.Clientset, error) {
	k8sClientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create k8s client set: %w", err)
	}

	return k8sClientSet, nil
}

func getK8sManagerOptions(watchNamespace string) manager.Options {
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
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
