package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/cloudogu/k8s-registry-lib/dogu"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/cloudogu/warp-assets/config"
	warp2 "github.com/cloudogu/warp-assets/controller"
	"github.com/cloudogu/warp-assets/logging"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
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
	if err := startManager(); err != nil {
		logger.Error(err, "manager produced an error")
		os.Exit(1)
	}
}

func startManager() error {
	logger.Info("Starting k8s-ces-assets warp discovery...")

	watchNamespace, err := config.ReadWatchNamespace()
	if err != nil {
		return fmt.Errorf("failed to read watch namespace: %w", err)
	}

	options := getK8sManagerOptions(watchNamespace)

	warpMenuManager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		return fmt.Errorf("failed to create new manager: %w", err)
	}

	/*
		eventRecorder := warpMenuManager.GetEventRecorderFor("k8s-ces-assets-nginx")

		clientset, err := getK8sClientSet(warpMenuManager.GetConfig())
		if err != nil {
			return fmt.Errorf("failed to create k8s client set: %w", err)
		}
		configMapInterface := clientset.CoreV1().ConfigMaps(watchNamespace)

		doguVersionRegistry := dogu.NewDoguVersionRegistry(configMapInterface)
		localDoguRepo := dogu.NewLocalDoguDescriptorRepository(configMapInterface)
		globalConfigRepo := repository.NewGlobalConfigRepository(configMapInterface)

		if err := handleWarpMenuCreation(warpMenuManager, doguVersionRegistry, localDoguRepo, watchNamespace, eventRecorder, globalConfigRepo); err != nil {
			return fmt.Errorf("failed to create warp menu creator: %w", err)
		}

	*/

	clientset, err := getK8sClientSet(warpMenuManager.GetConfig())
	if err != nil {
		return fmt.Errorf("failed to create k8s client set: %w", err)
	}

	configMapInterface := clientset.CoreV1().ConfigMaps(watchNamespace)
	globalConfigRepo := repository.NewGlobalConfigRepository(configMapInterface)
	doguVersionRegistry := dogu.NewDoguVersionRegistry(configMapInterface)
	localDoguRepo := dogu.NewLocalDoguDescriptorRepository(configMapInterface)

	client := warpMenuManager.GetClient()
	reconciler := warp2.NewWarpMenuReconciler(client, globalConfigRepo, doguVersionRegistry, localDoguRepo)
	err = reconciler.SetupWithManager(warpMenuManager)
	if err != nil {
		return fmt.Errorf("failure setup reconciler with manager: %w", err)
	}

	if err = startK8sManager(warpMenuManager); err != nil {
		return fmt.Errorf("failure at warp assets manager: %w", err)
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

func getK8sClientSet(config *rest.Config) (*kubernetes.Clientset, error) {
	k8sClientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client set: %w", err)
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

func handleWarpMenuCreation(k8sManager k8sManager, doguVersionRegistry warp2.DoguVersionRegistry, localDoguRepo warp2.LocalDoguRepo, namespace string, recorder record.EventRecorder, globalConfigRepo warp2.GlobalConfigRepository) error {
	warpMenuCreator := warp2.NewWarpMenuCreator(k8sManager.GetClient(), doguVersionRegistry, localDoguRepo, namespace, recorder, globalConfigRepo)

	if err := k8sManager.Add(warpMenuCreator); err != nil {
		return fmt.Errorf("failed to add warp menu creator as runnable to the manager: %w", err)
	}

	return nil
}
