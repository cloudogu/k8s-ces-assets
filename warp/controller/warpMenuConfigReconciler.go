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
	"github.com/go-logr/logr"
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

// WarpMenuConfigReconciler reconciles WarpMenuEntry CRs and the warp-config ConfigMap,
// rewriting the warp menu JSON file on every relevant change.
type WarpMenuConfigReconciler struct {
	client         client.Client
	eventRecorder  events.EventRecorder
	componentCRKey types.NamespacedName
	warpMenuPath   string
}

// NewWarpMenuReconciler creates a WarpMenuConfigReconciler.
// componentCRKey identifies the Component CR used as the anchor for operator-level events.
func NewWarpMenuReconciler(client client.Client, eventRecoder events.EventRecorder, warpMenuPath string, componentCRKey types.NamespacedName) *WarpMenuConfigReconciler {
	return &WarpMenuConfigReconciler{
		client:         client,
		eventRecorder:  eventRecoder,
		warpMenuPath:   warpMenuPath,
		componentCRKey: componentCRKey,
	}
}

// Reconcile rebuilds the warp menu from all WarpMenuEntry CRs and the default
// categories in the warp config. It is triggered by changes to either resource type.
func (r *WarpMenuConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, rErr error) {
	logger := log.FromContext(ctx)
	logger.Info("Starting WarpMenuConfigReconciler reconcile", "resource", req.Name, "namespace", req.Namespace)

	var validationErrs map[string]error
	defer func() {
		rErr = r.updateWarpCRStatus(ctx, req, rErr, validationErrs)
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

	warpMenuEntries, validationErrs := r.mapWarpMenuCRItemsToEntriesWithCategory(warpCrEntries.Items)
	if len(validationErrs) != 0 {
		r.emitGlobalWarning(ctx, reasonInvalidEntry, "one or more warpCRs are invalid")

		loggableErrs := make(map[string]string, len(validationErrs))
		for crName, vErr := range validationErrs {
			loggableErrs[crName] = vErr.Error()
		}

		logger.Info("one or more warpCRs are invalid; skipping them for warp menu",
			"invalidCount", len(validationErrs),
			"validationErrors", loggableErrs,
		)
	}

	defaultCategories := warpMenuConfiguration.DefaultCategories
	categories := defaultCategories.InsertEntries(warpMenuEntries)

	if wErr := r.writeWarpMenuFile(categories, logger); wErr != nil {
		return ctrl.Result{}, r.handleOperatorError(ctx, reasonFailedWriteMenu, "failed to write warp menu file", wErr)
	}

	r.emitGlobalNormal(ctx, reasonMenuUpdated, "warp menu entries have been updated.")
	logger.Info("Successfully updated warp menu.")

	return ctrl.Result{}, nil
}

// SetupWithManager registers the reconciler to watch WarpMenuEntry CRs and the
// warp-config ConfigMap. ConfigMap changes are mapped to a synthetic reconcile request.
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

// writeWarpMenuFile atomically replaces the warp menu JSON file by writing to a
// temporary file in the same directory and renaming it into place.
func (r *WarpMenuConfigReconciler) writeWarpMenuFile(categories domain.Categories, logger logr.Logger) error {
	finalPath := r.warpMenuPath + "/menu.json"

	tmpWarpFile, err := r.writeWarpMenuToTempFile(categories, logger)
	if err != nil {
		return fmt.Errorf("failed to write warp menu to temporary file: %w", err)
	}

	tmpWarpFilePath := tmpWarpFile.Name()
	if rErr := os.Rename(tmpWarpFilePath, finalPath); rErr != nil {
		_ = os.Remove(tmpWarpFilePath)

		return fmt.Errorf("failed to move temporary file to %s: %w", finalPath, rErr)
	}

	return nil
}

// writeWarpMenuToTempFile marshals categories as indented JSON into a temporary file.
// The file is closed before returning; callers may only use the returned handle for its name.
func (r *WarpMenuConfigReconciler) writeWarpMenuToTempFile(categories domain.Categories, logger logr.Logger) (*os.File, error) {
	tmpFile, err := os.OpenFile(r.warpMenuPath+"/menu.json.tmp", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file in %s: %w", r.warpMenuPath, err)
	}

	defer func() {
		if cErr := tmpFile.Close(); cErr != nil {
			logger.Error(cErr, "failed to close temp warp menu file", "path", tmpFile.Name())
		}
	}()

	encoder := json.NewEncoder(tmpFile)
	encoder.SetIndent("", "  ")

	if eErr := encoder.Encode(categories); eErr != nil {
		_ = os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed encode warp menu to temporary file: %w", eErr)
	}

	return tmpFile, nil
}

// updateWarpCRStatus writes condition updates to the WarpMenuEntry CR that triggered
// the reconcile. It is a no-op when the trigger was a ConfigMap change.
// If both a status-write error and a reconcile error occur, they are joined.
func (r *WarpMenuConfigReconciler) updateWarpCRStatus(ctx context.Context, reconcileReq ctrl.Request, reconcileErr error, validationErrs map[string]error) (uErr error) {
	// Reconcile hasn't been triggered by a warp cr
	if reconcileReq.Name == config.WarpConfigMap {
		return reconcileErr
	}

	defer func() {
		if reconcileErr != nil {
			uErr = errors.Join(uErr, reconcileErr)
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

		return nil
	}

	mErr, invalid := validationErrs[reconcileReq.Name]
	if invalid {
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

		return nil
	}

	visibleCondition := r.createVisibleStatusCondition(entry.Spec.Disabled, entry.Generation)
	readyCondition := r.createSuccessfulStatusCondition(entry.Generation)

	if statusError := r.updateStatusCondition(ctx, entry, visibleCondition, readyCondition); statusError != nil {
		return wrapUpdateStatusError(statusError)
	}

	r.eventRecorder.Eventf(entry, nil, corev1.EventTypeNormal, reasonMenuUpdated, actionReconcile, "WarpMenuEntry has been applied successfully")

	return nil
}

// updateStatusCondition applies conditions to the entry's status and persists the update.
// It skips the API call when no condition actually changed.
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

// createSuccessfulStatusCondition returns a ConditionReady=True condition.
func (r *WarpMenuConfigReconciler) createSuccessfulStatusCondition(generation int64) v1.Condition {
	return v1.Condition{
		Type:               warpmenu.ConditionReady,
		Status:             v1.ConditionTrue,
		ObservedGeneration: generation,
		Reason:             reasonMenuUpdated,
		Message:            "Warp menu entry has successfully been synced",
	}
}

// createVisibleStatusCondition returns a ConditionVisible condition reflecting whether the entry is disabled.
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
		condition.Reason = warpmenu.ReasonEntryRendered
		condition.Message = "Warp menu entry has been rendered."
	}
	return condition
}

// createErrorStatusCondition returns a ConditionReady=False condition with the given message.
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

// warpConfigChangeReconcile maps a ConfigMap change to a synthetic reconcile request
// whose Name equals config.WarpConfigMap, used as a sentinel in updateWarpCRStatus
// to skip per-entry status updates for ConfigMap-triggered reconcile runs.
func warpConfigChangeReconcile(ctx context.Context, obj client.Object) []reconcile.Request {
	log.FromContext(ctx).Info("warp config changed - creating a reconcile request to recreate all warp entries")

	// We don't have to list all entries and reconcile them because one reconciliation recreates the complete warp menu.
	// If a resource is not found, the reconciler recreates the warp menu, too.
	reconcileRequests := []reconcile.Request{{
		NamespacedName: types.NamespacedName{
			Name:      config.WarpConfigMap,
			Namespace: obj.GetNamespace(),
		}}}
	return reconcileRequests
}

// mapWarpMenuCRItemsToEntriesWithCategory converts WarpMenuEntry CRs to domain entries.
// Disabled entries are silently skipped; invalid entries are recorded in the returned error map keyed by CR name.
func (r *WarpMenuConfigReconciler) mapWarpMenuCRItemsToEntriesWithCategory(warpCREntries []warpmenu.WarpMenuEntry) (domain.EntriesWithCategory, map[string]error) {
	domainEntries := make(domain.EntriesWithCategory, 0, len(warpCREntries))
	validationErrs := make(map[string]error)

	for _, warpCR := range warpCREntries {
		if warpCR.Spec.Disabled {
			continue
		}

		domainEntry, mErr := mapWarpCRToEntryWithCategory(warpCR)
		if mErr != nil {
			r.eventRecorder.Eventf(&warpCR, nil, corev1.EventTypeWarning, reasonInvalidEntry, actionReconcile, "Failed to validate due to: %v", mErr)
			validationErrs[warpCR.GetName()] = fmt.Errorf("failed to map warp entry %q: %w", warpCR.GetName(), mErr)

			continue
		}

		domainEntries = append(domainEntries, domainEntry)
	}

	return domainEntries, validationErrs
}

// mapWarpCRToEntryWithCategory validates and maps a single WarpMenuEntry CR to a domain entry.
// All validation errors are collected and returned together.
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
			Localization: domain.LocalizationMap{
				domain.LocaleDe: crEntry.Spec.DisplayName.DE,
				domain.LocaleEn: crEntry.Spec.DisplayName.EN,
			},
			Href:   pathStr,
			Target: domain.TARGET_SELF,
		},
	}, nil
}

// validatePath checks that pathStr is a server-relative path: at least 2 characters,
// parseable as a URL, not an absolute URL (has scheme), and not a protocol-relative URL (has host).
func validatePath(pathStr string) error {
	if len(pathStr) < 2 {
		return fmt.Errorf("must have a length of at least 2")
	}

	pathURL, err := url.Parse(pathStr)
	if err != nil {
		return fmt.Errorf("unable to parse path to url: %w", err)
	}

	if pathURL.IsAbs() {
		return fmt.Errorf("path needs to be relative")
	}

	if pathURL.Host != "" {
		return fmt.Errorf("path must not contain a host")
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

// handleOperatorError emits a warning event on the Component CR and returns err wrapped with msg.
func (r *WarpMenuConfigReconciler) handleOperatorError(ctx context.Context, reason, msg string, err error) error {
	r.emitGlobalWarning(ctx, reason, msg)
	return fmt.Errorf("%s: %w", msg, err)
}
