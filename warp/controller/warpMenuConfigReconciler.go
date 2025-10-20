package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cloudogu/warp-assets/config"
	"github.com/cloudogu/warp-assets/controller/types"

	//"github.com/cloudogu/warp-assets/config"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const globalConfigMapName = "global-config"

type WarpMenuConfigReconciler struct {
	client              client.Client
	globalConfigRepo    GlobalConfigRepository
	doguVersionRegistry DoguVersionRegistry
	localDoguRepo       LocalDoguRepo
}

func NewWarpMenuReconciler(
	client client.Client,
	globalConfigRepo GlobalConfigRepository,
	doguVersionRegistry DoguVersionRegistry,
	localDoguRepo LocalDoguRepo,
) *WarpMenuConfigReconciler {
	return &WarpMenuConfigReconciler{
		client:              client,
		globalConfigRepo:    globalConfigRepo,
		doguVersionRegistry: doguVersionRegistry,
		localDoguRepo:       localDoguRepo,
	}
}

func (r *WarpMenuConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	//configMap := &corev1.ConfigMap{}
	//err := r.client.Get(ctx, req.NamespacedName, configMap)

	// check if config maps were created, deleted or updated ?

	warpMenuConfiguration, err := config.ReadConfiguration(ctx, r.client, req.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("read warp menu configuration: %w", err)
	}

	categories, err := r.createCategories(ctx, warpMenuConfiguration)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("create categories: %w", err)
	}

	err = r.writeWarpMenuJson(categories)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("write warp menu json file: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *WarpMenuConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		WithEventFilter(globalConfigPredicate()).
		Complete(r)
}

// TODO: filter dogu-spec-*
// TODO: filter global-config; only key "externals" ?
// TODO: filter k8s-ces-warp-config? It contains external links
func globalConfigPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.TypedCreateEvent[client.Object]) bool {
			return e.Object.GetName() == globalConfigMapName
		},
		DeleteFunc: func(e event.TypedDeleteEvent[client.Object]) bool {
			return e.Object.GetName() == globalConfigMapName
		},
		UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			return e.ObjectOld.GetName() == globalConfigMapName
		},
		GenericFunc: func(e event.TypedGenericEvent[client.Object]) bool {
			return e.Object.GetName() == globalConfigMapName
		},
	}
}

func (r *WarpMenuConfigReconciler) createCategories(ctx context.Context, warpMenuConfiguration *config.Configuration) (types.Categories, error) {
	configReader := NewConfigReader(
		warpMenuConfiguration,
		r.globalConfigRepo,
		r.doguVersionRegistry,
		r.localDoguRepo,
	)

	return configReader.Read(ctx, warpMenuConfiguration)
}

func (r *WarpMenuConfigReconciler) writeWarpMenuJson(categories types.Categories) error {
	jsonData, err := json.Marshal(categories)
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

	_, err = file.WriteString(string(jsonData))
	if err != nil {
		return fmt.Errorf("failed to write json data: %w", err)
	}

	return nil
}
