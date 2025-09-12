package controller

import (
	"context"
	"fmt"
	"github.com/cloudogu/ces-commons-lib/dogu"
	libconfig "github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/cloudogu/warp-assets/config"
	types3 "github.com/cloudogu/warp-assets/controller/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	types2 "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	warpMenuUpdateEventReason        = "WarpMenu"
	errorOnWarpMenuUpdateEventReason = "ErrUpdateWarpMenu"
)

// Watcher is used to watch a registry and for every change he reads from the registry a specific config path,
// build warp menu categories and writes them to a configmap.
type Watcher struct {
	configuration    *config.Configuration
	registryToWatch  DoguVersionRegistry
	globalConfigRepo GlobalConfigRepository
	k8sClient        client.Client
	ConfigReader     Reader
	namespace        string
	eventRecorder    eventRecorder
}

// NewWatcher creates a new Watcher instance to build the warp menu
func NewWatcher(ctx context.Context, k8sClient client.Client, doguVersionRegistry DoguVersionRegistry, localDoguRepo LocalDoguRepo, namespace string, recorder eventRecorder, globalConfigRepo GlobalConfigRepository) (*Watcher, error) {
	warpConfig, err := config.ReadConfiguration(ctx, k8sClient, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to Read configuration: %w", err)
	}

	reader := &ConfigReader{
		globalConfigRepo:    globalConfigRepo,
		doguVersionRegistry: doguVersionRegistry,
		localDoguRepo:       localDoguRepo,
		configuration:       warpConfig,
		doguConverter:       &types3.DoguConverter{},
		externalConverter:   &types3.ExternalConverter{},
	}

	return &Watcher{
		configuration:    warpConfig,
		registryToWatch:  doguVersionRegistry,
		k8sClient:        k8sClient,
		namespace:        namespace,
		ConfigReader:     reader,
		eventRecorder:    recorder,
		globalConfigRepo: globalConfigRepo,
	}, nil
}

// Run creates the warp menu and update the menu whenever a relevant configuration key was changed
func (w *Watcher) Run(ctx context.Context) error {
	// trigger the warp-menu creation once on startup
	err := w.execute(ctx)
	if err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "error creating warp-menu")
	}

	for _, source := range w.configuration.Sources {
		switch source.Type {
		case "dogus":
			w.startVersionRegistryWatch(ctx)
		case "externals":
			w.startGlobalConfigDirectoryWatch(ctx, source.Path)
		default:
			// do nothing
		}
	}

	return nil
}

func (w *Watcher) startGlobalConfigDirectoryWatch(ctx context.Context, sourcePath string) {
	ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("start global config watcher for source [%s]", sourcePath))
	configKey := libconfig.Key(sourcePath)

	filter := libconfig.DirectoryFilter(configKey)
	globalConfigWatchChannel, err := w.globalConfigRepo.Watch(ctx, filter)
	if err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "failed to create global config watch for path %q", sourcePath)
		return
	}

	go func() {
		w.handleGlobalConfigUpdates(ctx, globalConfigWatchChannel)
	}()
}

func (w *Watcher) handleGlobalConfigUpdates(ctx context.Context, globalConfigWatchChannel <-chan repository.GlobalConfigWatchResult) {
	for {
		select {
		case <-ctx.Done():
			ctrl.LoggerFrom(ctx).Info("context done - stop global config watch for warp generation")
			return
		case result, open := <-globalConfigWatchChannel:
			if !open {
				ctrl.LoggerFrom(ctx).Info("global config watch channel canceled - stop watch for warp generation")
				return
			}
			if result.Err != nil {
				ctrl.LoggerFrom(ctx).Error(result.Err, "global config watch channel error for warp generation")
				continue
			}
			// Trigger refresh. Content of the result is not needed
			err := w.execute(ctx)
			if err != nil {
				ctrl.LoggerFrom(ctx).Error(err, "failed to update entries from global config in warp menu")
			}
		}
	}
}

func (w *Watcher) startVersionRegistryWatch(ctx context.Context) {
	ctrl.LoggerFrom(ctx).Info("start version registry watcher for source type dogu")
	versionChannel, doguVersionWatchError := w.registryToWatch.WatchAllCurrent(ctx)
	if doguVersionWatchError != nil {
		ctrl.LoggerFrom(ctx).Error(doguVersionWatchError, "failed to create dogu version registry watch")
		return
	}

	go func() {
		w.handleDoguVersionUpdates(ctx, versionChannel)
	}()
}

func (w *Watcher) handleDoguVersionUpdates(ctx context.Context, versionChannel <-chan dogu.CurrentVersionsWatchResult) {
	for {
		select {
		case <-ctx.Done():
			ctrl.LoggerFrom(ctx).Info("context done - stop dogu version registry watch for warp generation")
			return
		case result, open := <-versionChannel:
			if !open {
				ctrl.LoggerFrom(ctx).Info("dogu version watch channel canceled - stop watch")
				return
			}
			if result.Err != nil {
				ctrl.LoggerFrom(ctx).Error(result.Err, "dogu version watch channel error")
				continue
			}
			// Trigger refresh. Content of the result is not needed
			err := w.execute(ctx)
			if err != nil {
				ctrl.LoggerFrom(ctx).Error(err, "failed to update dogus in warp menu")
			}
		}
	}
}

func (w *Watcher) execute(ctx context.Context) error {
	deployment := &appsv1.Deployment{}
	err := w.k8sClient.Get(ctx, types2.NamespacedName{Name: "k8s-ces-assets-nginx", Namespace: w.namespace}, deployment)
	if err != nil {
		return fmt.Errorf("warp update: failed to get deployment [%s]: %w", "k8s-ces-assets-nginx", err)
	}

	categories, err := w.ConfigReader.Read(ctx, w.configuration)
	if err != nil {
		w.eventRecorder.Eventf(deployment, corev1.EventTypeWarning, errorOnWarpMenuUpdateEventReason, "Updating warp menu failed: %w", err)
		return fmt.Errorf("error during read: %w", err)
	}
	ctrl.Log.Info(fmt.Sprintf("All found Categories: %v", categories))
	err = w.jsonWriter(categories)
	if err != nil {
		w.eventRecorder.Eventf(deployment, corev1.EventTypeWarning, errorOnWarpMenuUpdateEventReason, "Updating warp menu failed: %w", err)
		return fmt.Errorf("failed to write warp menu as json: %w", err)
	}
	w.eventRecorder.Event(deployment, corev1.EventTypeNormal, warpMenuUpdateEventReason, "Warp menu updated.")
	return nil
}

func (w *Watcher) jsonWriter(data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal warp data: %w", err)
	}

	path, err := config.ReadWarpPath()
	if err != nil {
		return fmt.Errorf("failed to get warp directory: %w", err)
	}

	file, err := os.Create(path + "/menu.json")
	if err != nil {
		return fmt.Errorf("failed to create warp.json: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	// Daten schreiben
	_, err = file.WriteString(string(jsonData))
	if err != nil {
		return fmt.Errorf("failed to write json data: %w", err)
	}

	return nil
}
