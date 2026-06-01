package controller

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	warpmenu "github.com/cloudogu/k8s-warp-menu-entry-lib/api/v1"
	"github.com/cloudogu/warp-assets/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

const (
	testDeploymentName = "aDeployment"
	testNamespace      = "aNamespace"
)

func TestWarpMenuReconcile(t *testing.T) {

	t.Run("should create menu entries for warp menu entries", func(t *testing.T) {
		firstEntry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		clientMock := newClientBuilder(t).
			WithRuntimeObjects(
				&appsv1.Deployment{
					ObjectMeta: ctrl.ObjectMeta{Name: testDeploymentName, Namespace: testNamespace},
				},
				getConfigMap(t, config.Configuration{}),
				&warpmenu.WarpMenuEntryList{
					Items: []warpmenu.WarpMenuEntry{
						firstEntry,
						buildWarpMenuEntry("dogu2", "Admin", "/dogu_2", "Dogu 2", "Dogu 2 en", false),
					},
				},
			).
			WithStatusSubresource(&firstEntry).
			Build()

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
		devAppsExpectedWarpMenuEntries := []WarpMenuEntry{
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
		assert.ElementsMatch(t, devAppsExpectedWarpMenuEntries, devAppsWarpMenuCategory.Entries)

		adminWarpMenuCategory, found := findCategoryByTitle(warpMenuCategories, "Admin")
		assert.True(t, found)
		adminExpectedWarpMenuEntries := []WarpMenuEntry{
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
		assert.ElementsMatch(t, adminExpectedWarpMenuEntries, adminWarpMenuCategory.Entries)

		verifyWarpMenuStatus(t, err, clientMock, request)

	})

	t.Run("Status update should work for  warp menu entries that already have a ready status", func(t *testing.T) {
		firstEntry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		_ = updateCRStatus(&firstEntry)
		clientMock := newClientBuilder(t).
			WithRuntimeObjects(
				&appsv1.Deployment{
					ObjectMeta: ctrl.ObjectMeta{Name: testDeploymentName, Namespace: testNamespace},
				},
				getConfigMap(t, config.Configuration{}),
				&warpmenu.WarpMenuEntryList{
					Items: []warpmenu.WarpMenuEntry{
						firstEntry,
					},
				},
			).
			WithStatusSubresource(&firstEntry).
			Build()

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
		devAppsExpectedWarpMenuEntries := []WarpMenuEntry{
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
		assert.ElementsMatch(t, devAppsExpectedWarpMenuEntries, devAppsWarpMenuCategory.Entries)

		verifyWarpMenuStatus(t, err, clientMock, request)
	})

	t.Run("should create menu entries when reconcile is called for warp menu entry deletion", func(t *testing.T) {
		firstEntry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		clientMock := newClientBuilder(t).
			WithRuntimeObjects(
				&appsv1.Deployment{
					ObjectMeta: ctrl.ObjectMeta{Name: testDeploymentName, Namespace: testNamespace},
				},
				getConfigMap(t, config.Configuration{}),
				&warpmenu.WarpMenuEntryList{
					Items: []warpmenu.WarpMenuEntry{
						firstEntry,
						buildWarpMenuEntry("dogu2", "Admin", "/dogu_2", "Dogu 2", "Dogu 2 en", false),
					},
				},
			).
			WithStatusSubresource(&firstEntry).
			Build()

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
		devAppsExpectedWarpMenuEntries := []WarpMenuEntry{
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
		assert.ElementsMatch(t, devAppsExpectedWarpMenuEntries, devAppsWarpMenuCategory.Entries)

		adminWarpMenuCategory, found := findCategoryByTitle(warpMenuCategories, "Admin")
		assert.True(t, found)
		adminExpectedWarpMenuEntries := []WarpMenuEntry{
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
		assert.ElementsMatch(t, adminExpectedWarpMenuEntries, adminWarpMenuCategory.Entries)

	})

}

func verifyWarpMenuStatus(t *testing.T, err error, clientMock client.WithWatch, request ctrl.Request) {
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
