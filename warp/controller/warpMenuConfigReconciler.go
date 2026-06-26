package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"

	component "github.com/cloudogu/k8s-component-lib/api/v1"
	warpmenu "github.com/cloudogu/k8s-warp-menu-entry-lib/api/v1"
	"github.com/cloudogu/warp-assets/config"
	domain "github.com/cloudogu/warp-assets/controller/types"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	actionReconcile = "ReconcileWarpMenu"

	reasonFailedReadConfig        = "FailedReadConfig"
	reasonInvalidConfig           = "InvalidConfig"
	reasonFailedGetEntry          = "FailedGetEntry"
	reasonFailedListEntries       = "FailedListEntries"
	reasonInvalidEntry            = "InvalidEntry"
	reasonFailedWriteMenu         = "FailedWriteMenu"
	reasonFailedUpdateStatusEntry = "FailedUpdateStatusWarpCR"
	reasonMenuGenerationFailed    = "MenuGenerationFailed"
	reasonMenuUpdated             = "WarpMenuUpdated"
)

type WarpMenuConfigReconciler struct {
	client         client.Client
	eventRecorder  events.EventRecorder
	componentCRKey types.NamespacedName
	warpMenuPath   string
}

func NewWarpMenuReconciler(client client.Client, eventRecoder events.EventRecorder, warpMenuPath string, componentCRKey types.NamespacedName) *WarpMenuConfigReconciler {
	return &WarpMenuConfigReconciler{
		client:         client,
		eventRecorder:  eventRecoder,
		warpMenuPath:   warpMenuPath,
		componentCRKey: componentCRKey,
	}
}

func (r *WarpMenuConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, rErr error) {
	logger := log.FromContext(ctx)
	logger.Info(fmt.Sprintf("Starting WarpMenuConfigReconciler reconcile for %s in namespace %s ...", req.Name, req.Namespace))

	defer func() {
		rErr = r.updateWarpCRStatus(ctx, req, rErr)
	}()

	warpMenuConfiguration, err := config.ReadConfiguration(ctx, r.client, req.Namespace)
	if err != nil {
		return ctrl.Result{}, r.handleOperatorError(ctx, reasonFailedReadConfig, "failed to read warp configuration", err)
	}

	if warpMenuConfiguration.LogErr != nil {
		logger.Error(
			r.handleOperatorError(ctx, reasonInvalidConfig, "warp config contains invalid entries", warpMenuConfiguration.LogErr),
			"invalid warp menu configuration",
		)
	}

	warpCrEntries := &warpmenu.WarpMenuEntryList{}
	err = r.client.List(ctx, warpCrEntries, client.InNamespace(req.Namespace))
	if err != nil {
		return ctrl.Result{}, r.handleOperatorError(ctx, reasonFailedListEntries, "failed to list warp menu entry CRs", err)
	}

	warpMenuEntries, mErr := r.mapWarpMenuCRItemsToEntriesWithCategory(warpCrEntries.Items)
	if mErr != nil {
		r.emitGlobalWarning(ctx, reasonInvalidEntry, "one or more warpCRs are invalid")
		logger.Info("one or more warpCRs are invalid: skipping them for warp menu", "error", mErr)
	}

	defaultCategories := warpMenuConfiguration.DefaultCategories
	categories := defaultCategories.InsertEntries(warpMenuEntries)

	if wErr := r.writeWarpMenuFile(categories); wErr != nil {
		return ctrl.Result{}, r.handleOperatorError(ctx, reasonFailedWriteMenu, "failed to write warp menu file", wErr)
	}

	r.emitGlobalNormal(ctx, reasonMenuUpdated, "warp menu entries have been updated.")
	logger.Info("Successfully updated warp menu.")

	return ctrl.Result{}, nil
}

func (r *WarpMenuConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&warpmenu.WarpMenuEntry{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(warpConfigChangeReconcile),
			builder.WithPredicates(config.WarpConfigMapPredicate()),
		).
		Complete(r)
}

func (r *WarpMenuConfigReconciler) writeWarpMenuFile(categories domain.Categories) error {
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

func (r *WarpMenuConfigReconciler) updateWarpCRStatus(ctx context.Context, reconcileReq ctrl.Request, reconcileErr error) (uErr error) {
	// Reconcile hasn't been triggered by a warp cr
	if reconcileReq.Name == config.WarpConfigMap {
		return reconcileErr
	}

	defer func() {
		if reconcileErr != nil {
			// always prefer error from reconciler
			uErr = reconcileErr
		}
	}()

	entry := &warpmenu.WarpMenuEntry{}
	err := r.client.Get(ctx, reconcileReq.NamespacedName, entry)
	// nothing to do, as CR does not exist
	if apierrors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		return r.handleOperatorError(ctx, reasonFailedGetEntry, fmt.Sprintf("failed to get warp entry %q for status update", reconcileReq.Name), err)
	}

	wrapUpdateStatusError := func(statusErr error) error {
		return r.handleOperatorError(ctx, reasonFailedUpdateStatusEntry, fmt.Sprintf("failed to update status conditions for entry %q", entry.Name), statusErr)
	}

	if reconcileErr != nil {
		condition := r.createErrorStatusCondition(entry.Generation, "an internal operator error occurred")
		if statusError := r.updateStatusCondition(ctx, entry, condition); statusError != nil {
			return wrapUpdateStatusError(statusError)
		}
	}

	_, mErr := mapWarpCRToEntryWithCategory(*entry)
	if mErr != nil {
		// warp cr is invalid and is skipped by the operator
		r.eventRecorder.Eventf(entry, nil, corev1.EventTypeWarning, reasonInvalidEntry, actionReconcile,
			"warp cr is invalid: %v", mErr)

		visibleCondition := v1.Condition{
			Type:               warpmenu.ConditionVisible,
			Status:             v1.ConditionFalse,
			ObservedGeneration: entry.Generation,
			Reason:             reasonInvalidEntry,
			Message:            "Warp menu entry is not rendered, because it is invalid.",
		}

		readyCondition := v1.Condition{
			Type:               warpmenu.ConditionReady,
			Status:             v1.ConditionFalse,
			ObservedGeneration: entry.Generation,
			Reason:             reasonInvalidEntry,
			Message:            "Warp menu entry is not ready, because of invalid entries",
		}

		if statusError := r.updateStatusCondition(ctx, entry, visibleCondition, readyCondition); statusError != nil {
			return wrapUpdateStatusError(statusError)
		}
	}

	visibleCondition := r.createVisibleStatusCondition(entry.Spec.Disabled, entry.Generation)
	readyCondition := r.createSuccessfulStatusCondition(entry.Generation)

	if statusError := r.updateStatusCondition(ctx, entry, visibleCondition, readyCondition); statusError != nil {
		return wrapUpdateStatusError(statusError)
	}

	return nil
}

func (r *WarpMenuConfigReconciler) updateStatusCondition(ctx context.Context, entry *warpmenu.WarpMenuEntry, conditions ...v1.Condition) error {
	anyConditionChanged := false

	for _, condition := range conditions {
		if meta.SetStatusCondition(&entry.Status.Conditions, condition) {
			// If at least one condition actually changed, flip flag
			anyConditionChanged = true
		}
	}

	if !anyConditionChanged {
		return nil
	}

	if err := r.client.Status().Update(ctx, entry); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	return nil
}

func (r *WarpMenuConfigReconciler) createSuccessfulStatusCondition(generation int64) v1.Condition {
	return v1.Condition{
		Type:               warpmenu.ConditionReady,
		Status:             v1.ConditionTrue,
		ObservedGeneration: generation,
		Reason:             reasonMenuUpdated,
		Message:            "Warp menu entry has successfully been synced",
	}
}

func (r *WarpMenuConfigReconciler) createVisibleStatusCondition(disabled bool, generation int64) v1.Condition {
	condition := v1.Condition{
		Type:               warpmenu.ConditionVisible,
		Status:             v1.ConditionTrue,
		ObservedGeneration: generation,
	}
	if disabled {
		condition.Status = v1.ConditionFalse
		condition.Reason = warpmenu.ReasonEntryHidden
		condition.Message = "Warp menu entry has been hidden, because it is disabled."
	} else {
		condition.Status = v1.ConditionTrue
		condition.Reason = warpmenu.ReasonEntryRendered
		condition.Message = "Warp menu entry has been rendered."
	}
	return condition
}

func (r *WarpMenuConfigReconciler) createErrorStatusCondition(generation int64, errorMessage string) v1.Condition {
	condition := v1.Condition{
		Type:               warpmenu.ConditionReady,
		ObservedGeneration: generation,
		Status:             v1.ConditionFalse,
		Reason:             reasonMenuGenerationFailed,
		Message:            errorMessage,
	}
	return condition
}

func warpConfigChangeReconcile(ctx context.Context, obj client.Object) []reconcile.Request {
	log.FromContext(ctx).Info(fmt.Sprintf("warp config changed - creating a reconcile request to recreate all warp entries:  Object triggering the reconcile: [Namespace: %s ,Name: %s]  %v", obj.GetNamespace(), obj.GetName(), ctx))

	// We don't have to list all entries and reconcile them because one reconciliation recreates the complete warp menu.
	// If a resource is not found, the reconciler recreates the warp menu, too.
	reconcileRequests := []reconcile.Request{{
		NamespacedName: types.NamespacedName{
			Name:      config.WarpConfigMap,
			Namespace: obj.GetNamespace(),
		}}}
	return reconcileRequests
}

func (r *WarpMenuConfigReconciler) mapWarpMenuCRItemsToEntriesWithCategory(warpCREntries []warpmenu.WarpMenuEntry) (domain.EntriesWithCategory, error) {
	domainEntries := make(domain.EntriesWithCategory, 0, len(warpCREntries))

	var mErrs []error
	for _, warpCR := range warpCREntries {
		if warpCR.Spec.Disabled {
			continue
		}

		domainEntry, mErr := mapWarpCRToEntryWithCategory(warpCR)
		if mErr != nil {
			r.eventRecorder.Eventf(&warpCR, nil, corev1.EventTypeWarning, reasonInvalidEntry, actionReconcile, "Failed to validate due to: %v", mErr)
			mErrs = append(mErrs, fmt.Errorf("failed to map warp entry %q: %w", warpCR.GetName(), mErr))

			continue
		}

		domainEntries = append(domainEntries, domainEntry)
	}

	return domainEntries, errors.Join(mErrs...)
}

func mapWarpCRToEntryWithCategory(crEntry warpmenu.WarpMenuEntry) (domain.EntryWithCategory, error) {
	var vErrs []error

	if len(crEntry.Spec.Category) == 0 {
		vErrs = append(vErrs, fmt.Errorf("category is empty"))
	}

	pathStr := crEntry.Spec.Path
	if vErr := validatePath(pathStr); vErr != nil {
		vErrs = append(vErrs, fmt.Errorf("invalid path %q: %w", pathStr, vErr))
	}

	if len(vErrs) > 0 {
		return domain.EntryWithCategory{}, errors.Join(vErrs...)
	}

	return domain.EntryWithCategory{
		Category: crEntry.Spec.Category,
		Entry: domain.Entry{
			Identifier: crEntry.GetName(),
			DisplayName: domain.TranslationMap{
				domain.LocaleDe: crEntry.Spec.DisplayName.DE,
				domain.LocaleEn: crEntry.Spec.DisplayName.EN,
			},
			Href:   pathStr,
			Target: domain.TARGET_SELF,
		},
	}, nil
}

func validatePath(pathStr string) error {
	if len(pathStr) < 2 {
		return fmt.Errorf("must have a length of at least 2")
	}

	pathURL, err := url.Parse(pathStr)
	if err != nil {
		return fmt.Errorf("unable to parse path to url: %w", err)
	}

	if !pathURL.IsAbs() {
		return fmt.Errorf("path needs to be relative")
	}

	return nil
}

// emitGlobalWarning fires a warning on the Component CR anchor.
func (r *WarpMenuConfigReconciler) emitGlobalWarning(ctx context.Context, reason, msg string) {
	comp := &component.Component{}
	if err := r.client.Get(ctx, r.componentCRKey, comp); err != nil {
		return
	}
	r.eventRecorder.Eventf(comp, nil, corev1.EventTypeWarning, reason, actionReconcile, msg)
}

// emitGlobalNormal fires a normal event on the Component CR anchor.
func (r *WarpMenuConfigReconciler) emitGlobalNormal(ctx context.Context, reason, msg string) {
	comp := &component.Component{}
	if err := r.client.Get(ctx, r.componentCRKey, comp); err != nil {
		return
	}
	r.eventRecorder.Eventf(comp, nil, corev1.EventTypeNormal, reason, actionReconcile, msg)
}

func (r *WarpMenuConfigReconciler) handleOperatorError(ctx context.Context, reason, msg string, err error) error {
	r.emitGlobalWarning(ctx, reason, msg)
	return fmt.Errorf("%s: %w", msg, err)
}
