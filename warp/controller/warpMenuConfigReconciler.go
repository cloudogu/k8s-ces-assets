package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cloudogu/warp-assets/config"
	"github.com/cloudogu/warp-assets/controller/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	types2 "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	globalConfigMapName              = "global-config"
	warpMenuUpdateEventReason        = "WarpMenu"
	errorOnWarpMenuUpdateEventReason = "ErrUpdateWarpMenu"
)

type WarpMenuConfigReconciler struct {
	client              k8sClient
	globalConfigRepo    GlobalConfigRepository
	doguVersionRegistry DoguVersionRegistry
	localDoguRepo       LocalDoguRepo
	eventRecorder       eventRecorder
	warpMenuPath        string
	deploymentName      string
}

func NewWarpMenuReconciler(client k8sClient, globalConfigRepo GlobalConfigRepository, doguVersionRegistry DoguVersionRegistry, localDoguRepo LocalDoguRepo, eventRecoder eventRecorder, warpMenuPath string, deploymentName string) *WarpMenuConfigReconciler {
	return &WarpMenuConfigReconciler{
		client:              client,
		globalConfigRepo:    globalConfigRepo,
		doguVersionRegistry: doguVersionRegistry,
		localDoguRepo:       localDoguRepo,
		eventRecorder:       eventRecoder,
		warpMenuPath:        warpMenuPath,
		deploymentName:      deploymentName,
	}
}

func (r *WarpMenuConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("WarpMenuConfigReconciler reconcile()")

	deployment := &appsv1.Deployment{}
	err := r.client.Get(ctx, types2.NamespacedName{Name: r.deploymentName, Namespace: req.Namespace}, deployment)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("warp update: failed to get deployment [%s]: %w", "k8s-ces-assets-nginx", err)
	}

	warpMenuConfiguration, err := config.ReadConfiguration(ctx, r.client, req.Namespace)
	if err != nil {
		r.eventRecorder.Eventf(deployment, corev1.EventTypeWarning, errorOnWarpMenuUpdateEventReason, "Reading warp menu config failed: %w", err)
		return ctrl.Result{}, fmt.Errorf("read warp menu configuration: %w", err)
	}

	categories, err := r.createCategories(ctx, warpMenuConfiguration)
	if err != nil {
		r.eventRecorder.Eventf(deployment, corev1.EventTypeWarning, errorOnWarpMenuUpdateEventReason, "Creating warp menu categories failed: %w", err)
		return ctrl.Result{}, fmt.Errorf("create categories: %w", err)
	}

	err = r.writeWarpMenuFile(categories)
	if err != nil {
		r.eventRecorder.Eventf(deployment, corev1.EventTypeWarning, errorOnWarpMenuUpdateEventReason, "Writing warp menu file failed: %w", err)
		return ctrl.Result{}, fmt.Errorf("write warp menu file: %w", err)
	}

	r.eventRecorder.Event(deployment, corev1.EventTypeNormal, warpMenuUpdateEventReason, "Warp menu updated.")
	return ctrl.Result{}, nil
}

func (r *WarpMenuConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		WithEventFilter(eventFilterPredicate()).
		Complete(r)
}

func eventFilterPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.TypedCreateEvent[client.Object]) bool {
			return isWatchedConfigMap(e.Object.GetName())
		},
		DeleteFunc: func(e event.TypedDeleteEvent[client.Object]) bool {
			return isWatchedConfigMap(e.Object.GetName())
		},
		UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			return isWatchedConfigMap(e.ObjectOld.GetName())
		},
		GenericFunc: func(e event.TypedGenericEvent[client.Object]) bool {
			return isWatchedConfigMap(e.Object.GetName())
		},
	}
}

func isWatchedConfigMap(configMapName string) bool {
	isDoguSpecConfigMap := strings.HasPrefix(configMapName, "dogu-spec-")
	return isDoguSpecConfigMap || configMapName == globalConfigMapName || configMapName == config.WarpConfigMap
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

func (r *WarpMenuConfigReconciler) writeWarpMenuFile(categories types.Categories) error {
	jsonData, err := json.Marshal(categories)
	if err != nil {
		return fmt.Errorf("failed to marshal warp data: %w", err)
	}

	path := r.warpMenuPath + "/menu.json"
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %s %w", path, err)
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
