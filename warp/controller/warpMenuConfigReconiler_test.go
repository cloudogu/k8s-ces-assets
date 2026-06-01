package controller

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/cloudogu/ces-commons-lib/dogu"
	"github.com/cloudogu/cesapp-lib/core"
	config2 "github.com/cloudogu/k8s-registry-lib/config"
	warpmenu "github.com/cloudogu/k8s-warp-menu-entry-lib/api/v1"
	"github.com/cloudogu/warp-assets/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

	t.Run("should create menu entries for dogus", func(t *testing.T) {
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
			WithStatusSubresource(&firstEntry)

		warpMenuPath := t.TempDir()
		eventRecorderMock := newMockEventRecorder(t)

		eventRecorderMock.EXPECT().Event(mock.Anything, corev1.EventTypeNormal, warpMenuUpdateEventReason, "Warp menu updated.")

		reconciler := NewWarpMenuReconciler(clientMock.Build(), eventRecorderMock, warpMenuPath, testDeploymentName)

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

	})

	t.Run("should not create a menu entry for a dogu that doesn't have the right tag", func(t *testing.T) {
		clientMock := newMockK8sClient(t)
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		doguVersionRegistryMock := NewMockDoguVersionRegistry(t)
		localDoguRepo := NewMockLocalDoguRepo(t)
		warpMenuPath := t.TempDir()
		eventRecorderMock := newMockEventRecorder(t)

		eventRecorderMock.EXPECT().Event(mock.Anything, corev1.EventTypeNormal, warpMenuUpdateEventReason, "Warp menu updated.")

		warpMenuConfig := config.Configuration{
			Sources: []config.Source{
				{
					Path: "/dogu",
					Type: "dogus",
					Tag:  "show_in_warp_menu",
				},
			},
		}
		mockExpectGetWarpMenuConfig(t, clientMock, warpMenuConfig)

		globalConfig := config2.CreateGlobalConfig(config2.Entries{})
		globalConfigRepoMock.EXPECT().Get(mock.Anything).Return(globalConfig, nil)

		dogus := []*core.Dogu{
			{
				Name:        "repo/dogu_1",
				Version:     "1.0.0-1",
				DisplayName: "Dogu 1",
				Description: "Dogu 1 Description",
				Category:    "DevApps",
				Tags:        []string{"tag_does_not_match_tag_in_warp_menu_config"},
			},
		}

		doguSimpleVersionNames, simpleVersionNameToDoguMap := newSimpleNameToDoguMap(t, dogus)
		doguVersionRegistryMock.EXPECT().GetCurrentOfAll(mock.Anything).Return(doguSimpleVersionNames, nil)
		localDoguRepo.EXPECT().GetAll(mock.Anything, doguSimpleVersionNames).Return(simpleVersionNameToDoguMap, nil)

		reconciler := NewWarpMenuReconciler(clientMock, eventRecorderMock, warpMenuPath, testDeploymentName)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "aNamespace", Name: "aConfigMap"}}
		_, err := reconciler.Reconcile(context.Background(), request)
		require.NoError(t, err)

		warpMenuCategories := parseWarpMenuCategoriesFromJsonFile(t, warpMenuPath)
		assert.Equal(t, 0, len(warpMenuCategories))
	})
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

func mockExpectGetWarpMenuConfig(t *testing.T, clientMock *mockK8sClient, warpMenuConfig config.Configuration) {
	clientMock.EXPECT().
		Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1.ConfigMap")).
		Run(func(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) {
			warpMenuConfigAsString, err := yaml.Marshal(warpMenuConfig)
			require.NoError(t, err)

			configMap := obj.(*corev1.ConfigMap)
			data := map[string]string{
				"warp": string(warpMenuConfigAsString),
			}
			configMap.Data = data
		}).
		Return(nil)
}

func findCategoryByTitle(warpMenuCategories []WarpMenuCategory, title string) (WarpMenuCategory, bool) {
	for _, cat := range warpMenuCategories {
		if cat.Title == title {
			return cat, true
		}
	}
	return WarpMenuCategory{}, false
}

func newSimpleNameToDoguMap(t *testing.T, dogus []*core.Dogu) ([]dogu.SimpleNameVersion, map[dogu.SimpleNameVersion]*core.Dogu) {
	var nameVersions []dogu.SimpleNameVersion
	doguMap := make(map[dogu.SimpleNameVersion]*core.Dogu)
	for _, d := range dogus {
		version, err := core.ParseVersion(d.Version)
		require.NoError(t, err)
		nameVersion := dogu.SimpleNameVersion{
			Name:    dogu.SimpleName(d.Name),
			Version: version,
		}
		nameVersions = append(nameVersions, nameVersion)
		doguMap[nameVersion] = d
	}

	return nameVersions, doguMap
}
