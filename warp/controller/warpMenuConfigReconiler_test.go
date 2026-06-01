package controller

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	warpmenu "github.com/cloudogu/k8s-warp-menu-entry-lib/api/v1"
	"github.com/cloudogu/warp-assets/config"
	warpassetstypes "github.com/cloudogu/warp-assets/controller/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/yaml"
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
		eventRecorderMock := newMockEventRecorder(t)

		eventRecorderMock.EXPECT().Event(mock.Anything, corev1.EventTypeNormal, warpMenuUpdateEventReason, "Warp menu updated.")

		reconciler := NewWarpMenuReconciler(clientMock, eventRecorderMock, warpMenuPath, testDeploymentName)

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

		verifyWarpMenuStatus(t, err, clientMock, request, false)

	})

	t.Run("Should not create warp menu entries for the warp menu entries that are disabled", func(t *testing.T) {
		firstEntry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", true)
		clientMock := getClientMock(t, []warpmenu.WarpMenuEntry{firstEntry})

		warpMenuPath := t.TempDir()
		eventRecorderMock := newMockEventRecorder(t)

		eventRecorderMock.EXPECT().Event(mock.Anything, corev1.EventTypeNormal, warpMenuUpdateEventReason, "Warp menu updated.")

		reconciler := NewWarpMenuReconciler(clientMock, eventRecorderMock, warpMenuPath, testDeploymentName)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "aNamespace", Name: firstEntry.Name}}
		_, err := reconciler.Reconcile(context.Background(), request)
		require.NoError(t, err)

		warpMenuCategories := parseWarpMenuCategoriesFromJsonFile(t, warpMenuPath)
		assert.Equal(t, 0, len(warpMenuCategories))

		_, found := findCategoryByTitle(warpMenuCategories, "DevApps")
		assert.False(t, found)
		verifyWarpMenuStatus(t, err, clientMock, request, true)
	})

	t.Run("Status update should work for warp menu entries that already have a ready status", func(t *testing.T) {
		firstEntry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		_ = updateCRStatus(&firstEntry)
		clientMock := getClientMock(t, []warpmenu.WarpMenuEntry{firstEntry})

		warpMenuPath := t.TempDir()
		eventRecorderMock := newMockEventRecorder(t)

		eventRecorderMock.EXPECT().Event(mock.Anything, corev1.EventTypeNormal, warpMenuUpdateEventReason, "Warp menu updated.")

		reconciler := NewWarpMenuReconciler(clientMock, eventRecorderMock, warpMenuPath, testDeploymentName)

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
	})

	t.Run("should create menu entries when reconcile is called for warp menu entry deletion", func(t *testing.T) {
		firstEntry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		secondEntry := buildWarpMenuEntry("dogu2", "Admin", "/dogu_2", "Dogu 2", "Dogu 2 en", false)

		clientMock := getClientMock(t, []warpmenu.WarpMenuEntry{firstEntry, secondEntry})

		warpMenuPath := t.TempDir()
		eventRecorderMock := newMockEventRecorder(t)

		eventRecorderMock.EXPECT().Event(mock.Anything, corev1.EventTypeNormal, warpMenuUpdateEventReason, "Warp menu updated.")

		reconciler := NewWarpMenuReconciler(clientMock, eventRecorderMock, warpMenuPath, testDeploymentName)

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

	})

	t.Run("Reconcile should fail if unable to list warp menu entries", func(t *testing.T) {
		firstEntry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", true)
		clientMock := getClientMockWithListError(t, []warpmenu.WarpMenuEntry{firstEntry})

		warpMenuPath := t.TempDir()
		eventRecorderMock := newMockEventRecorder(t)

		eventRecorderMock.EXPECT().Eventf(mock.Anything, corev1.EventTypeWarning, errorOnWarpMenuUpdateEventReason, "Reading warp menu entry CRs failed: %v", mock.Anything)

		reconciler := NewWarpMenuReconciler(clientMock, eventRecorderMock, warpMenuPath, testDeploymentName)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "aNamespace", Name: firstEntry.Name}}
		_, err := reconciler.Reconcile(context.Background(), request)
		assert.EqualError(t, err, "read warp menu entry CRs: Simulating api error")

	})

	t.Run("Reconcile should fail if deployment is not available", func(t *testing.T) {
		firstEntry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		_ = updateCRStatus(&firstEntry)
		clientMock := getClientMock(t, []warpmenu.WarpMenuEntry{firstEntry})

		warpMenuPath := t.TempDir()
		eventRecorderMock := newMockEventRecorder(t)

		eventRecorderMock.EXPECT().Eventf(mock.Anything, corev1.EventTypeWarning, errorOnWarpMenuUpdateEventReason, "Failed to get the deployment: %v", mock.Anything)
		reconciler := NewWarpMenuReconciler(clientMock, eventRecorderMock, warpMenuPath, testDeploymentName+"wrong")

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: firstEntry.Name}}
		_, err := reconciler.Reconcile(context.Background(), request)
		assert.EqualError(t, err, "warp update: failed to get deployment [aDeploymentwrong]: deployments.apps \"aDeploymentwrong\" not found")

	})

	t.Run("Reconcile should fail if configuration is not available", func(t *testing.T) {
		firstEntry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		_ = updateCRStatus(&firstEntry)
		clientMock := getClientMockWithoutConfigMap(t, []warpmenu.WarpMenuEntry{firstEntry})

		warpMenuPath := t.TempDir()
		eventRecorderMock := newMockEventRecorder(t)
		eventRecorderMock.EXPECT().Eventf(mock.Anything, corev1.EventTypeWarning, errorOnWarpMenuUpdateEventReason, "Reading warp menu config failed: %v", mock.Anything)

		reconciler := NewWarpMenuReconciler(clientMock, eventRecorderMock, warpMenuPath, testDeploymentName)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: firstEntry.Name}}
		_, err := reconciler.Reconcile(context.Background(), request)
		assert.EqualError(t, err, "read warp menu configuration: failed to get warp menu configmap: configmaps \"k8s-ces-warp-config\" not found")

	})
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

func TestWriteWarpMenuFile(t *testing.T) {

	t.Run("should give error because of wrong warpmenupath", func(t *testing.T) {

		scheme := runtime.NewScheme()
		err := warpmenu.AddToScheme(scheme)
		assert.NoError(t, err)
		assert.NoError(t, err)

		reconciler := NewWarpMenuReconciler(nil, nil, "nonexistingpath", testDeploymentName)
		categories := &warpassetstypes.Categories{}
		err = reconciler.writeWarpMenuFile(*categories)

		assert.EqualError(t, err, "failed to create file: nonexistingpath/menu.json open nonexistingpath/menu.json: no such file or directory")
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

func verifyWarpMenuStatus(t *testing.T, err error, clientMock client.WithWatch, request ctrl.Request, disabled bool) {
	updatedWarpMenuEntry := &warpmenu.WarpMenuEntry{}
	err = clientMock.Get(context.Background(), request.NamespacedName, updatedWarpMenuEntry)
	assert.NoError(t, err)
	assert.NotEmpty(t, updatedWarpMenuEntry.Status.Conditions)
	assert.Equal(t, 1, len(updatedWarpMenuEntry.Status.Conditions))

	expectedCondition := metav1.Condition{
		Type:               warpmenu.ConditionReady,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: updatedWarpMenuEntry.Status.Conditions[0].LastTransitionTime,
		Reason:             warpmenu.ReasonEntryRendered,
		Message:            "Warp menu entry has been rendered.",
	}
	if disabled {
		expectedCondition.Reason = warpmenu.ReasonEntryHidden
		expectedCondition.Message = "Warp menu entry has been hidden, because it is disabled."
	}
	assert.Equal(t, expectedCondition, updatedWarpMenuEntry.Status.Conditions[0])
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

func getConfigMap(t *testing.T, warpMenuConfig config.Configuration) *corev1.ConfigMap {
	warpMenuConfigAsString, err := yaml.Marshal(warpMenuConfig)
	require.NoError(t, err)

	configMap := corev1.ConfigMap{}
	data := map[string]string{
		"warp": string(warpMenuConfigAsString),
	}
	configMap.Name = "k8s-ces-warp-config"
	configMap.Namespace = testNamespace
	configMap.Data = data
	return &configMap
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
