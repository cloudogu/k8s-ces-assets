package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	warpmenu "github.com/cloudogu/k8s-warp-menu-entry-lib/api/v1"
	"github.com/cloudogu/warp-assets/config"
	"github.com/cloudogu/warp-assets/controller/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types2 "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	warpMenuUpdateEventReason        = "WarpMenu"
	errorOnWarpMenuUpdateEventReason = "ErrUpdateWarpMenu"
)

type WarpMenuConfigReconciler struct {
	client         k8sClient
	eventRecorder  eventRecorder
	warpMenuPath   string
	deploymentName string
}

func NewWarpMenuReconciler(client k8sClient, eventRecoder eventRecorder, warpMenuPath string, deploymentName string) *WarpMenuConfigReconciler {
	return &WarpMenuConfigReconciler{
		client:         client,
		eventRecorder:  eventRecoder,
		warpMenuPath:   warpMenuPath,
		deploymentName: deploymentName,
	}
}

func (r *WarpMenuConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("WarpMenuConfigReconciler reconcile()")

	deployment := &appsv1.Deployment{}
	err := r.client.Get(ctx, types2.NamespacedName{Name: r.deploymentName, Namespace: req.Namespace}, deployment)
	if err != nil {
		r.eventRecorder.Eventf(deployment, corev1.EventTypeWarning, errorOnWarpMenuUpdateEventReason, "Failed to get the deployment: %v", err)
		return ctrl.Result{}, fmt.Errorf("warp update: failed to get deployment [%s]: %w", r.deploymentName, err)
	}

	warpMenuConfiguration, err := config.ReadConfiguration(ctx, r.client, req.Namespace)
	if err != nil {
		r.eventRecorder.Eventf(deployment, corev1.EventTypeWarning, errorOnWarpMenuUpdateEventReason, "Reading warp menu config failed: %v", err)
		return ctrl.Result{}, fmt.Errorf("read warp menu configuration: %w", err)
	}

	warpMenuEntries := &warpmenu.WarpMenuEntryList{}
	err = r.client.List(ctx, warpMenuEntries, client.InNamespace(req.Namespace))
	if err != nil {
		r.eventRecorder.Eventf(deployment, corev1.EventTypeWarning, errorOnWarpMenuUpdateEventReason, "Reading warp menu entry CRs failed: %v", err)
		return ctrl.Result{}, fmt.Errorf("read warp menu entry CRs: %w", err)
	}

	categories, err := r.createCategories(&warpMenuConfiguration.Order, warpMenuEntries)
	if err != nil {
		r.eventRecorder.Eventf(deployment, corev1.EventTypeWarning, errorOnWarpMenuUpdateEventReason, "Creating warp menu categories failed: %v", err)
		return ctrl.Result{}, fmt.Errorf("create categories: %w", err)
	}

	err = r.writeWarpMenuFile(categories)
	if err != nil {
		r.eventRecorder.Eventf(deployment, corev1.EventTypeWarning, errorOnWarpMenuUpdateEventReason, "Writing warp menu file failed: %v", err)
		return ctrl.Result{}, fmt.Errorf("write warp menu file: %w", err)
	}

	err = r.updateWarpMenuEntryStatus(ctx, warpMenuEntries, req)
	if err != nil {
		r.eventRecorder.Eventf(deployment, corev1.EventTypeWarning, errorOnWarpMenuUpdateEventReason, "Updating warp menu entry status for %s failed: %v", req.Name, err)
		return ctrl.Result{}, fmt.Errorf("update status of %s: %w", req.Name, err)
	}

	r.eventRecorder.Event(deployment, corev1.EventTypeNormal, warpMenuUpdateEventReason, "Warp menu updated.")
	return ctrl.Result{}, nil
}

func (r *WarpMenuConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&warpmenu.WarpMenuEntry{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}

func (r *WarpMenuConfigReconciler) createCategories(order *config.Order, menuEntries *warpmenu.WarpMenuEntryList) (types.Categories, error) {
	menuBuilder := WarpMenuBuilder{order: *order}
	return menuBuilder.buildCategories(menuEntries)
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

func (r *WarpMenuConfigReconciler) updateWarpMenuEntryStatus(ctx context.Context, entries *warpmenu.WarpMenuEntryList, req ctrl.Request) error {
	for _, entry := range entries.Items {
		if entry.Name == req.Name && entry.Namespace == req.Namespace {
			condition := r.createStatusCondition(entry.Spec.Disabled, entry.Generation)
			meta.SetStatusCondition(&entry.Status.Conditions, condition)
			err := r.client.Status().Update(ctx, &entry)
			return err
		}
	}
	log.FromContext(ctx).Info("Warp menu entry not found, the warp menu entry may be deleted.")
	return nil
}

func (r *WarpMenuConfigReconciler) createStatusCondition(disabled bool, generation int64) v1.Condition {
	var condition v1.Condition
	if disabled {
		condition = v1.Condition{
			Reason:  warpmenu.ReasonEntryHidden,
			Message: "Warp menu entry has been hidden, because it is disabled.",
		}
	} else {
		condition = v1.Condition{
			Reason:  warpmenu.ReasonEntryRendered,
			Message: "Warp menu entry has been rendered.",
		}
	}
	condition.Type = warpmenu.ConditionReady
	condition.Status = v1.ConditionTrue
	condition.LastTransitionTime = v1.NewTime(time.Now())
	condition.ObservedGeneration = generation
	return condition
}
