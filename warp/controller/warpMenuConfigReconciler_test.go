package controller

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	warpmenu "github.com/cloudogu/k8s-warp-menu-entry-lib/api/v1"
	"github.com/cloudogu/warp-assets/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	testDeploymentName = "aDeployment"
	testNamespace      = "aNamespace"
)

func TestWarpMenuReconcile(t *testing.T) {

	t.Run("should create menu entries for warp menu entries", func(t *testing.T) {
		firstEntry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		secondEntry := buildWarpMenuEntry("dogu2", "Admin", "/dogu_2", "Dogu 2", "Dogu 2 en", false)
		clientMock := getClientMock(t, []warpmenu.WarpMenuEntry{firstEntry, secondEntry})

		warpMenuPath := t.TempDir()

		fakeRecorder := events.NewFakeRecorder(5)

		reconciler := NewWarpMenuReconciler(clientMock, fakeRecorder, warpMenuPath, testDeploymentName)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "aNamespace", Name: firstEntry.Name}}
		_, err := reconciler.Reconcile(context.Background(), request)
		require.NoError(t, err)

		warpMenuCategories := parseWarpMenuCategoriesFromJsonFile(t, warpMenuPath)
		assert.Equal(t, 2, len(warpMenuCategories))

		devAppsWarpMenuCategory, found := findCategoryByTitle(warpMenuCategories, "DevApps")
		assert.True(t, found)
		devAppsExpectedWarpMenuEntries := getExpectedWarpMenuEntry1()
		assert.ElementsMatch(t, devAppsExpectedWarpMenuEntries, devAppsWarpMenuCategory.Entries)

		adminWarpMenuCategory, found := findCategoryByTitle(warpMenuCategories, "Admin")
		assert.True(t, found)
		adminExpectedWarpMenuEntries := getExpectedWarpMenuEntry2()
		assert.ElementsMatch(t, adminExpectedWarpMenuEntries, adminWarpMenuCategory.Entries)

		// only the status of the reconciled entry should be changed
		verifyWarpMenuStatus(t, err, clientMock, request, false)
		verifyNoChangeToStatusCondition(t, clientMock, &secondEntry)

		validateRecorderEvents(t, fakeRecorder, "Warp menu updated.", warpMenuUpdateEventReason)
	})

	t.Run("Should not create warp menu entries for the warp menu entries that are disabled", func(t *testing.T) {
		firstEntry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", true)
		clientMock := getClientMock(t, []warpmenu.WarpMenuEntry{firstEntry})

		warpMenuPath := t.TempDir()

		fakeRecorder := events.NewFakeRecorder(5)

		reconciler := NewWarpMenuReconciler(clientMock, fakeRecorder, warpMenuPath, testDeploymentName)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "aNamespace", Name: firstEntry.Name}}
		_, err := reconciler.Reconcile(context.Background(), request)
		require.NoError(t, err)

		warpMenuCategories := parseWarpMenuCategoriesFromJsonFile(t, warpMenuPath)
		assert.Equal(t, 0, len(warpMenuCategories))

		_, found := findCategoryByTitle(warpMenuCategories, "DevApps")
		assert.False(t, found)
		verifyWarpMenuStatus(t, err, clientMock, request, true)
		validateRecorderEvents(t, fakeRecorder, "Warp menu updated.", warpMenuUpdateEventReason)
	})

	t.Run("Status update should work for warp menu entries that already have a ready status", func(t *testing.T) {
		firstEntry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		_ = updateCRStatus(&firstEntry)
		clientMock := getClientMock(t, []warpmenu.WarpMenuEntry{firstEntry})

		warpMenuPath := t.TempDir()
		fakeRecorder := events.NewFakeRecorder(5)

		reconciler := NewWarpMenuReconciler(clientMock, fakeRecorder, warpMenuPath, testDeploymentName)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "aNamespace", Name: firstEntry.Name}}
		_, err := reconciler.Reconcile(context.Background(), request)
		require.NoError(t, err)

		warpMenuCategories := parseWarpMenuCategoriesFromJsonFile(t, warpMenuPath)
		assert.Equal(t, 1, len(warpMenuCategories))

		devAppsWarpMenuCategory, found := findCategoryByTitle(warpMenuCategories, "DevApps")
		assert.True(t, found)
		devAppsExpectedWarpMenuEntries := getExpectedWarpMenuEntry1()
		assert.ElementsMatch(t, devAppsExpectedWarpMenuEntries, devAppsWarpMenuCategory.Entries)

		verifyWarpMenuStatus(t, err, clientMock, request, false)
		validateRecorderEvents(t, fakeRecorder, "Warp menu updated.", warpMenuUpdateEventReason)
	})

	t.Run("should create menu entries when reconcile is called for warp menu entry deletion", func(t *testing.T) {
		firstEntry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		secondEntry := buildWarpMenuEntry("dogu2", "Admin", "/dogu_2", "Dogu 2", "Dogu 2 en", false)

		clientMock := getClientMock(t, []warpmenu.WarpMenuEntry{firstEntry, secondEntry})

		warpMenuPath := t.TempDir()
		fakeRecorder := events.NewFakeRecorder(5)

		reconciler := NewWarpMenuReconciler(clientMock, fakeRecorder, warpMenuPath, testDeploymentName)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "aNamespace", Name: "DeletedWarpMenuEntry"}}
		_, err := reconciler.Reconcile(context.Background(), request)
		require.NoError(t, err)

		warpMenuCategories := parseWarpMenuCategoriesFromJsonFile(t, warpMenuPath)
		assert.Equal(t, 2, len(warpMenuCategories))

		devAppsWarpMenuCategory, found := findCategoryByTitle(warpMenuCategories, "DevApps")
		assert.True(t, found)
		devAppsExpectedWarpMenuEntries := getExpectedWarpMenuEntry1()
		assert.ElementsMatch(t, devAppsExpectedWarpMenuEntries, devAppsWarpMenuCategory.Entries)

		adminWarpMenuCategory, found := findCategoryByTitle(warpMenuCategories, "Admin")
		assert.True(t, found)
		adminExpectedWarpMenuEntries := getExpectedWarpMenuEntry2()
		assert.ElementsMatch(t, adminExpectedWarpMenuEntries, adminWarpMenuCategory.Entries)

		validateRecorderEvents(t, fakeRecorder, "Warp menu updated.", warpMenuUpdateEventReason)

	})

	t.Run("Reconcile should fail if unable to list warp menu entries", func(t *testing.T) {
		firstEntry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", true)
		clientMock := getClientMockWithListError(t, []warpmenu.WarpMenuEntry{firstEntry})

		warpMenuPath := t.TempDir()
		fakeRecorder := events.NewFakeRecorder(5)

		reconciler := NewWarpMenuReconciler(clientMock, fakeRecorder, warpMenuPath, testDeploymentName)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "aNamespace", Name: firstEntry.Name}}
		_, err := reconciler.Reconcile(context.Background(), request)
		assert.EqualError(t, err, "Reading warp menu entry CRs failed: Simulating api error")
		verifyFailedWarpMenuStatus(t, err, clientMock, request, "Reading warp menu entry CRs failed")
		validateRecorderEvents(t, fakeRecorder, "Reading warp menu entry CRs failed: Simulating api error", errorOnWarpMenuUpdateEventReason)

	})

	t.Run("Reconcile should fail if deployment is not available", func(t *testing.T) {
		firstEntry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		_ = updateCRStatus(&firstEntry)
		clientMock := getClientMock(t, []warpmenu.WarpMenuEntry{firstEntry})

		warpMenuPath := t.TempDir()
		fakeRecorder := events.NewFakeRecorder(5)

		reconciler := NewWarpMenuReconciler(clientMock, fakeRecorder, warpMenuPath, testDeploymentName+"wrong")

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: firstEntry.Name}}
		_, err := reconciler.Reconcile(context.Background(), request)
		assert.EqualError(t, err, "warp update: failed to get deployment [aDeploymentwrong]: deployments.apps \"aDeploymentwrong\" not found")
		verifyFailedWarpMenuStatus(t, err, clientMock, request, "warp update: failed to get deployment [aDeploymentwrong]")
	})

	t.Run("Reconcile should fail if warp menu configmap is not available", func(t *testing.T) {
		firstEntry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		_ = updateCRStatus(&firstEntry)
		clientMock := getClientMockWithoutConfigMap(t, []warpmenu.WarpMenuEntry{firstEntry})

		warpMenuPath := t.TempDir()
		fakeRecorder := events.NewFakeRecorder(5)

		reconciler := NewWarpMenuReconciler(clientMock, fakeRecorder, warpMenuPath, testDeploymentName)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: firstEntry.Name}}
		_, err := reconciler.Reconcile(context.Background(), request)
		assert.EqualError(t, err, "Reading warp menu config failed: failed to get warp menu configmap: configmaps \"k8s-ces-warp-config\" not found")
		verifyFailedWarpMenuStatus(t, err, clientMock, request, "Reading warp menu config failed")
		validateRecorderEvents(t, fakeRecorder, "Reading warp menu config failed: failed to get warp menu configmap: configmaps \"k8s-ces-warp-config\" not found", errorOnWarpMenuUpdateEventReason)

	})

	t.Run("Reconcile should fail if writing to WarpMenuFile fails", func(t *testing.T) {
		firstEntry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", true)
		clientMock := getClientMock(t, []warpmenu.WarpMenuEntry{firstEntry})

		fakeRecorder := events.NewFakeRecorder(5)

		reconciler := NewWarpMenuReconciler(clientMock, fakeRecorder, "nonexistingpath", testDeploymentName)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "aNamespace", Name: firstEntry.Name}}
		_, err := reconciler.Reconcile(context.Background(), request)
		assert.EqualError(t, err, "Writing warp menu file failed: failed to create file: nonexistingpath/menu.json open nonexistingpath/menu.json: no such file or directory")
		verifyFailedWarpMenuStatus(t, err, clientMock, request, "Writing warp menu file failed")
		validateRecorderEvents(t, fakeRecorder, "Writing warp menu file failed: failed to create file: nonexistingpath/menu.json open nonexistingpath/menu.json: no such file or directory", errorOnWarpMenuUpdateEventReason)
	})

	t.Run("Reconcile should fail if updating the WarpMenuEntry Status fails", func(t *testing.T) {
		firstEntry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", true)
		clientMock := getClientMockWithStatusUpdateError(t, []warpmenu.WarpMenuEntry{firstEntry})

		warpMenuPath := t.TempDir()
		fakeRecorder := events.NewFakeRecorder(5)

		//eventRecorderMock.EXPECT().Eventf(mock.Anything, corev1.EventTypeWarning, errorOnWarpMenuUpdateEventRe ason, "Updating warp menu entry status for %s failed: %v", mock.Anything, mock.Anything)

		reconciler := NewWarpMenuReconciler(clientMock, fakeRecorder, warpMenuPath, testDeploymentName)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "aNamespace", Name: firstEntry.Name}}
		_, err := reconciler.Reconcile(context.Background(), request)
		assert.EqualError(t, err, "update status of dogu1: mocked SubResourceClient error")
		validateRecorderEvents(t, fakeRecorder, "Updating warp menu entry status for dogu1 failed: mocked SubResourceClient error", errorOnWarpMenuUpdateEventReason)

	})
}

func validateRecorderEvents(t *testing.T, fakeRecorder *events.FakeRecorder, reason string, notes string) {
	select {
	case event := <-fakeRecorder.Events:
		assert.Contains(t, event, reason)
		assert.Contains(t, event, notes)
	default:
		t.Fatal("Expected an event to be recorded, but found none!")
	}
}

func TestSetupWithManager(t *testing.T) {

	t.Run("should setup the controller with manager", func(t *testing.T) {

		scheme := runtime.NewScheme()
		err := warpmenu.AddToScheme(scheme)
		assert.NoError(t, err)
		mgr, err := manager.New(&rest.Config{}, manager.Options{
			Scheme: scheme,
		})
		assert.NoError(t, err)

		reconciler := NewWarpMenuReconciler(nil, nil, "doesnotmatter", testDeploymentName)

		err = reconciler.SetupWithManager(mgr)
		assert.NoError(t, err)
	})
}

func TestMapConfigMapToWarpMenuEntries(t *testing.T) {

	t.Run("should call reconcile when WarpConfigMap is modified", func(t *testing.T) {

		watchNamespace, _ := os.LookupEnv("WATCH_NAMESPACE")
		defer os.Setenv("WATCH_NAMESPACE", watchNamespace)
		require.NoError(t, os.Setenv("WATCH_NAMESPACE", testNamespace))

		scheme := runtime.NewScheme()
		err := warpmenu.AddToScheme(scheme)
		assert.NoError(t, err)

		firstEntry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		secondEntry := buildWarpMenuEntry("dogu2", "Admin", "/dogu_2", "Dogu 2", "Dogu 2 en", false)
		clientMock := getClientMock(t, []warpmenu.WarpMenuEntry{firstEntry, secondEntry})

		reconciler := NewWarpMenuReconciler(clientMock, nil, "doesnotmatter", testDeploymentName)

		configmap := &corev1.ConfigMap{}
		configmap.Name = config.WarpConfigMap
		configmap.Namespace = testNamespace

		reconcile1 := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      firstEntry.Name,
				Namespace: firstEntry.Namespace,
			},
		}
		reconcile2 := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      secondEntry.Name,
				Namespace: secondEntry.Namespace,
			},
		}

		expectedReconcileRequest := []reconcile.Request{reconcile1, reconcile2}
		reconcileRequest := reconciler.mapConfigMapToWarpMenuEntries(context.Background(), configmap)

		assert.Equal(t, expectedReconcileRequest, reconcileRequest)
	})

	t.Run("should finish successfully when WarpConfigMap is modified and there are no warp menu entries", func(t *testing.T) {

		watchNamespace, _ := os.LookupEnv("WATCH_NAMESPACE")
		defer os.Setenv("WATCH_NAMESPACE", watchNamespace)
		require.NoError(t, os.Setenv("WATCH_NAMESPACE", testNamespace))

		scheme := runtime.NewScheme()
		err := warpmenu.AddToScheme(scheme)
		assert.NoError(t, err)

		clientMock := getClientMock(t, []warpmenu.WarpMenuEntry([]warpmenu.WarpMenuEntry(nil)))

		reconciler := NewWarpMenuReconciler(clientMock, nil, "doesnotmatter", testDeploymentName)

		configmap := &corev1.ConfigMap{}
		configmap.Name = config.WarpConfigMap
		configmap.Namespace = testNamespace

		reconcileRequest := reconciler.mapConfigMapToWarpMenuEntries(context.Background(), configmap)

		assert.Equal(t, []reconcile.Request([]reconcile.Request(nil)), reconcileRequest)
	})

	t.Run("should return null if there is an error getting the warp menu entry list when WarpConfigMap is modified ", func(t *testing.T) {

		watchNamespace, _ := os.LookupEnv("WATCH_NAMESPACE")
		defer os.Setenv("WATCH_NAMESPACE", watchNamespace)
		require.NoError(t, os.Setenv("WATCH_NAMESPACE", testNamespace))

		scheme := runtime.NewScheme()
		err := warpmenu.AddToScheme(scheme)
		assert.NoError(t, err)

		firstEntry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		secondEntry := buildWarpMenuEntry("dogu2", "Admin", "/dogu_2", "Dogu 2", "Dogu 2 en", false)
		clientMock := getClientMockWithListError(t, []warpmenu.WarpMenuEntry{firstEntry, secondEntry})

		reconciler := NewWarpMenuReconciler(clientMock, nil, "doesnotmatter", testDeploymentName)

		configmap := &corev1.ConfigMap{}
		configmap.Name = config.WarpConfigMap
		configmap.Namespace = testNamespace

		reconcileRequest := reconciler.mapConfigMapToWarpMenuEntries(context.Background(), configmap)

		assert.Equal(t, []reconcile.Request([]reconcile.Request(nil)), reconcileRequest)
	})
}

func getExpectedWarpMenuEntry2() []WarpMenuEntry {
	return []WarpMenuEntry{
		{
			Title:       "",
			DisplayName: "Dogu 2",
			Href:        "/dogu_2",
			Target:      "self",
			Localization: map[string]string{
				"de": "Dogu 2",
				"en": "Dogu 2 en",
			},
		},
	}
}

func getExpectedWarpMenuEntry1() []WarpMenuEntry {
	return []WarpMenuEntry{
		{
			Title:       "",
			DisplayName: "Dogu 1",
			Href:        "/dogu_1",
			Target:      "self",
			Localization: map[string]string{
				"de": "Dogu 1",
				"en": "Dogu 1 en",
			},
		},
	}
}

func verifyFailedWarpMenuStatus(t *testing.T, err error, clientMock client.WithWatch, request ctrl.Request, expectedErrorMessage string) {
	updatedWarpMenuEntry := &warpmenu.WarpMenuEntry{}
	err = clientMock.Get(context.Background(), request.NamespacedName, updatedWarpMenuEntry)
	assert.NoError(t, err)
	assert.NotEmpty(t, updatedWarpMenuEntry.Status.Conditions)
	assert.Equal(t, 1, len(updatedWarpMenuEntry.Status.Conditions))

	readyCondition := meta.FindStatusCondition(updatedWarpMenuEntry.Status.Conditions, warpmenu.ConditionReady)

	expectedCondition := &metav1.Condition{
		Type:               warpmenu.ConditionReady,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: readyCondition.LastTransitionTime,
		Reason:             reasonMenuGenerationFailed,
		Message:            expectedErrorMessage,
		ObservedGeneration: updatedWarpMenuEntry.Generation,
	}
	assert.Equal(t, expectedCondition, readyCondition)
}

func verifyWarpMenuStatus(t *testing.T, err error, clientMock client.WithWatch, request ctrl.Request, disabled bool) {
	updatedWarpMenuEntry := &warpmenu.WarpMenuEntry{}
	err = clientMock.Get(context.Background(), request.NamespacedName, updatedWarpMenuEntry)
	assert.NoError(t, err)
	assert.NotEmpty(t, updatedWarpMenuEntry.Status.Conditions)
	assert.Equal(t, 2, len(updatedWarpMenuEntry.Status.Conditions))

	actualReadyCondition := meta.FindStatusCondition(updatedWarpMenuEntry.Status.Conditions, warpmenu.ConditionReady)
	actualVisibleCondition := meta.FindStatusCondition(updatedWarpMenuEntry.Status.Conditions, conditionVisible)

	expectedReadyCondition := &metav1.Condition{
		Type:               warpmenu.ConditionReady,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: actualReadyCondition.LastTransitionTime,
		Reason:             reasonMenuGenerated,
		Message:            "Warp menu entry has successfully been synced",
		ObservedGeneration: updatedWarpMenuEntry.Generation,
	}
	expectedVisibleCondition := &metav1.Condition{
		Type:               conditionVisible,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: actualVisibleCondition.LastTransitionTime,
		Reason:             warpmenu.ReasonEntryRendered,
		Message:            "Warp menu entry has been rendered.",
		ObservedGeneration: updatedWarpMenuEntry.Generation,
	}
	if disabled {
		expectedVisibleCondition.Status = metav1.ConditionFalse
		expectedVisibleCondition.Reason = warpmenu.ReasonEntryHidden
		expectedVisibleCondition.Message = "Warp menu entry has been hidden, because it is disabled."
	}
	assert.Equal(t, expectedReadyCondition, actualReadyCondition)
	assert.Equal(t, expectedVisibleCondition, actualVisibleCondition)
}

func verifyNoChangeToStatusCondition(t *testing.T, clientMock client.WithWatch, secondWarpMenuEntry *warpmenu.WarpMenuEntry) {
	warpMenuEntry := &warpmenu.WarpMenuEntry{}
	err := clientMock.Get(context.Background(), types.NamespacedName{Namespace: testNamespace, Name: secondWarpMenuEntry.Name}, warpMenuEntry)
	assert.NoError(t, err)
	assert.Empty(t, warpMenuEntry.Status.Conditions)
}

func newClientBuilder(t *testing.T) *fake.ClientBuilder {
	scheme := runtime.NewScheme()
	err := appsv1.AddToScheme(scheme)
	require.NoError(t, err)
	err = corev1.AddToScheme(scheme)
	require.NoError(t, err)
	err = warpmenu.AddToScheme(scheme)
	require.NoError(t, err)
	return fake.NewClientBuilder().WithScheme(scheme)
}

func parseWarpMenuCategoriesFromJsonFile(t *testing.T, warpMenuPath string) []WarpMenuCategory {
	data, err := os.ReadFile(warpMenuPath + "/menu.json")
	require.NoError(t, err)

	warpMenuCategories := &[]WarpMenuCategory{}
	err = json.Unmarshal(data, warpMenuCategories)
	require.NoError(t, err)

	return *warpMenuCategories
}

func findCategoryByTitle(warpMenuCategories []WarpMenuCategory, title string) (WarpMenuCategory, bool) {
	for _, cat := range warpMenuCategories {
		if cat.Title == title {
			return cat, true
		}
	}
	return WarpMenuCategory{}, false
}

func updateCRStatus(cr *warpmenu.WarpMenuEntry) error {
	newCondition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             warpmenu.ReasonEntryRendered,
		Message:            "Warp menu entry has been rendered.",
		LastTransitionTime: metav1.Now(),
	}
	meta.SetStatusCondition(&cr.Status.Conditions, newCondition)
	return nil
}
