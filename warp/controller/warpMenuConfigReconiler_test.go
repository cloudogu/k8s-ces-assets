package controller

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/cloudogu/ces-commons-lib/dogu"
	"github.com/cloudogu/cesapp-lib/core"
	config2 "github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/warp-assets/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	types2 "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	testDeploymentName = "aDeployment"
	testNamespace      = "aNamespace"
)

func TestWarpMenuReconcile(t *testing.T) {

	t.Run("should create menu entries for dogus", func(t *testing.T) {
		clientMock := newMockK8sClient(t)
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		doguVersionRegistryMock := NewMockDoguVersionRegistry(t)
		localDoguRepo := NewMockLocalDoguRepo(t)
		warpMenuPath := t.TempDir()
		eventRecorderMock := newMockEventRecorder(t)

		mocksExpectWriteEvent(clientMock, eventRecorderMock)

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
				Tags:        []string{"show_in_warp_menu"},
			},
			{
				Name:        "repo/dogu_2",
				Version:     "2.0.0-1",
				DisplayName: "Dogu 2",
				Description: "Dogu 2 Description",
				Category:    "Admin",
				Tags:        []string{"show_in_warp_menu"},
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
		assert.Equal(t, 2, len(warpMenuCategories))

		devAppsWarpMenuCategory, found := findCategoryByTitle(warpMenuCategories, "DevApps")
		assert.True(t, found)
		devAppsExpectedWarpMenuEntries := []WarpMenuEntry{
			{
				Title:       "Dogu 1 Description",
				DisplayName: "Dogu 1",
				Href:        "/dogu_1",
				Target:      "self",
			},
		}
		assert.ElementsMatch(t, devAppsExpectedWarpMenuEntries, devAppsWarpMenuCategory.Entries)

		adminWarpMenuCategory, found := findCategoryByTitle(warpMenuCategories, "Admin")
		assert.True(t, found)
		adminExpectedWarpMenuEntries := []WarpMenuEntry{
			{
				Title:       "Dogu 2 Description",
				DisplayName: "Dogu 2",
				Href:        "/dogu_2",
				Target:      "self",
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

		mocksExpectWriteEvent(clientMock, eventRecorderMock)

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

func parseWarpMenuCategoriesFromJsonFile(t *testing.T, warpMenuPath string) []WarpMenuCategory {
	data, err := os.ReadFile(warpMenuPath + "/menu.json")
	require.NoError(t, err)

	warpMenuCategories := &[]WarpMenuCategory{}
	err = json.Unmarshal(data, warpMenuCategories)
	require.NoError(t, err)

	return *warpMenuCategories
}

func mockExpectGetWarpMenuConfig(t *testing.T, clientMock *mockK8sClient, warpMenuConfig config.Configuration) {
	clientMock.EXPECT().
		Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1.ConfigMap")).
		Run(func(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) {
			warpMenuConfigAsString, err := yaml.Marshal(warpMenuConfig)
			require.NoError(t, err)

			configMap := obj.(*v1.ConfigMap)
			data := map[string]string{
				"warp": string(warpMenuConfigAsString),
			}
			configMap.Data = data
		}).
		Return(nil)
}

func mocksExpectWriteEvent(clientMock *mockK8sClient, eventRecorderMock *mockEventRecorder) {
	clientMock.EXPECT().
		Get(mock.Anything, types2.NamespacedName{Name: testDeploymentName, Namespace: testNamespace}, mock.AnythingOfType("*v1.Deployment")).
		Return(nil)

	eventRecorderMock.EXPECT().Event(mock.Anything, v1.EventTypeNormal, warpMenuUpdateEventReason, "Warp menu updated.")

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
