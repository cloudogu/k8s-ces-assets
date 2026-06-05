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
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	types2 "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	warpMenuUpdateEventReason        = "WarpMenu"
	errorOnWarpMenuUpdateEventReason = "ErrUpdateWarpMenu"
	warpMenuUpdateEventAction        = "WarpMenuEntryReconcile"
	reasonMenuGenerationFailed       = "MenuGenerationFailed"
)

type WarpMenuConfigReconciler struct {
	client         client.Client
	eventRecorder  events.EventRecorder
	warpMenuPath   string
	deploymentName string
}

func NewWarpMenuReconciler(client client.Client, eventRecoder events.EventRecorder, warpMenuPath string, deploymentName string) *WarpMenuConfigReconciler {
	return &WarpMenuConfigReconciler{
		client:         client,
		eventRecorder:  eventRecoder,
		warpMenuPath:   warpMenuPath,
		deploymentName: deploymentName,
	}
}

func (r *WarpMenuConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info(fmt.Sprintf("Starting reconcile for %s in namespace %s ..", req.Name, req.Namespace))

	entry, err := r.loadEntryToReconcile(ctx, req, logger)
	if err != nil {
		return ctrl.Result{}, r.handleError(ctx, err, nil, nil, "warp update: failed to get entry [%s] to be reconciled", req.Name)
	}

	deployment := &appsv1.Deployment{}
	err = r.client.Get(ctx, types2.NamespacedName{Name: r.deploymentName, Namespace: req.Namespace}, deployment)
	if err != nil {
		return ctrl.Result{}, r.handleError(ctx, err, entry, nil, "warp update: failed to get deployment [%s]", r.deploymentName)
	}

	warpMenuConfiguration, err := config.ReadConfiguration(ctx, r.client, req.Namespace)
	if err != nil {
		return ctrl.Result{}, r.handleError(ctx, err, entry, deployment, "Reading warp menu config failed")
	}

	warpMenuEntries := &warpmenu.WarpMenuEntryList{}
	err = r.client.List(ctx, warpMenuEntries, client.InNamespace(req.Namespace))
	if err != nil {
		return ctrl.Result{}, r.handleError(ctx, err, entry, deployment, "Reading warp menu entry CRs failed")
	}

	categories := r.createCategories(warpMenuConfiguration, warpMenuEntries)

	err = r.writeWarpMenuFile(categories)
	if err != nil {
		return ctrl.Result{}, r.handleError(ctx, err, entry, deployment, "Writing warp menu file failed")
	}

	err = r.updateWarpMenuEntryStatus(ctx, entry, deployment)
	if err != nil {
		logger.Error(err, "error while reconciling")
		return ctrl.Result{}, fmt.Errorf("update status of %s: %w", req.Name, err)
	}

	r.eventRecorder.Eventf(deployment, entry, corev1.EventTypeNormal, warpMenuUpdateEventReason, warpMenuUpdateEventAction, "Warp menu updated.")
	logger.Info("Reconcile was successful")
	return ctrl.Result{}, nil
}

func (r *WarpMenuConfigReconciler) loadEntryToReconcile(ctx context.Context, req ctrl.Request, logger logr.Logger) (*warpmenu.WarpMenuEntry, error) {
	entry := &warpmenu.WarpMenuEntry{}
	err := r.client.Get(ctx, req.NamespacedName, entry)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Entry CR to be reconciled not found - probably deleted?", "name", req.Name)
			return nil, nil
		}
		return nil, err
	}
	return entry, nil
}

func (r *WarpMenuConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&warpmenu.WarpMenuEntry{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}

func (r *WarpMenuConfigReconciler) createCategories(configuration *config.Configuration, menuEntries *warpmenu.WarpMenuEntryList) types.Categories {
	return WarpMenuBuilder{order: configuration.Order}.buildCategories(menuEntries)
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

func (r *WarpMenuConfigReconciler) handleError(ctx context.Context, err error, entry *warpmenu.WarpMenuEntry, deployment *appsv1.Deployment, message string, a ...any) error {
	errorMessage := fmt.Sprintf(message, a...)
	if deployment != nil {
		r.eventRecorder.Eventf(deployment, entry, corev1.EventTypeWarning, errorOnWarpMenuUpdateEventReason, warpMenuUpdateEventAction, errorMessage+": %v", err)
	}
	if entry != nil {
		condition := r.createErrorStatusCondition(entry.Generation, errorMessage)
		statusError := r.updateStatusCondition(ctx, entry, condition, deployment)
		if statusError != nil {
			log.FromContext(ctx).Error(statusError, "error occurred while updating the error status condition for the entry : %v", entry)
		}
	}
	return fmt.Errorf(errorMessage+": %w", err)
}

func (r *WarpMenuConfigReconciler) updateWarpMenuEntryStatus(ctx context.Context, entry *warpmenu.WarpMenuEntry, deployment *appsv1.Deployment) error {
	if entry != nil {
		condition := r.createSuccessfulStatusCondition(entry.Spec.Disabled, entry.Generation)
		return r.updateStatusCondition(ctx, entry, condition, deployment)
	}
	return nil
}

func (r *WarpMenuConfigReconciler) updateStatusCondition(ctx context.Context, entry *warpmenu.WarpMenuEntry, condition v1.Condition, deployment *appsv1.Deployment) error {
	if meta.SetStatusCondition(&entry.Status.Conditions, condition) {
		newCondition := meta.FindStatusCondition(entry.Status.Conditions, condition.Type)
		if newCondition != nil {
			newCondition.LastTransitionTime = v1.NewTime(time.Now())
		}
	}
	err := r.client.Status().Update(ctx, entry)
	if err != nil {
		log.FromContext(ctx).Error(err, "Updating warp menu entry status failed", "name", entry.Name)
		if deployment != nil {
			r.eventRecorder.Eventf(deployment, entry, corev1.EventTypeWarning, errorOnWarpMenuUpdateEventReason, warpMenuUpdateEventAction, "Updating warp menu entry status for %s failed: %v", entry.Name, err)
		}
	}
	return err
}

func (r *WarpMenuConfigReconciler) createSuccessfulStatusCondition(disabled bool, generation int64) v1.Condition {
	condition := v1.Condition{
		Type:               warpmenu.ConditionReady,
		Status:             v1.ConditionTrue,
		ObservedGeneration: generation,
	}
	if disabled {
		condition.Reason = warpmenu.ReasonEntryHidden
		condition.Message = "Warp menu entry has been hidden, because it is disabled."
	} else {
		condition.Reason = warpmenu.ReasonEntryRendered
		condition.Message = "Warp menu entry has been rendered."
	}
	return condition
}

func (r *WarpMenuConfigReconciler) createErrorStatusCondition(generation int64, errorMessage string) v1.Condition {
	condition := v1.Condition{
		Type:               warpmenu.ConditionReady,
		ObservedGeneration: generation,
		LastTransitionTime: v1.NewTime(time.Now()),
		Status:             v1.ConditionFalse,
		Reason:             reasonMenuGenerationFailed,
		Message:            errorMessage,
	}
	return condition
}
