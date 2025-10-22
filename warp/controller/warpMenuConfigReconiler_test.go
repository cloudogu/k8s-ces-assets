package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/cloudogu/ces-commons-lib/dogu"
	"github.com/cloudogu/cesapp-lib/core"
	config2 "github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/warp-assets/config"
	"github.com/cloudogu/warp-assets/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/yaml"
)

func TestWarpMenuReconcile(t *testing.T) {
	checkEventFilterPredicate := func(configMapName string, shouldBeWatched bool) {
		configMap := newConfigMapWithName(configMapName)
		funcs := eventFilterPredicate()

		var testName string
		if shouldBeWatched {
			testName = fmt.Sprintf("should watch config map: '%s'", configMapName)
		} else {
			testName = fmt.Sprintf("should not watch config map: '%s'", configMapName)
		}

		t.Run(testName, func(t *testing.T) {
			assert.Equal(t, shouldBeWatched, funcs.Create(event.CreateEvent{Object: configMap}))
			assert.Equal(t, shouldBeWatched, funcs.Delete(event.DeleteEvent{Object: configMap}))
			assert.Equal(t, shouldBeWatched, funcs.Update(event.UpdateEvent{ObjectOld: configMap, ObjectNew: configMap}))
			assert.Equal(t, shouldBeWatched, funcs.Generic(event.GenericEvent{Object: configMap}))
		})
	}

	checkEventFilterPredicate("dogu-spec-redmine", true)
	checkEventFilterPredicate("dogu-spec-postgres", true)
	checkEventFilterPredicate(globalConfigMapName, true)
	checkEventFilterPredicate(config.WarpConfigMap, true)
	checkEventFilterPredicate("a-config-map", false)

	t.Run("should create menu entries configured in global config map", func(t *testing.T) {
		clientMock := newMockK8sClient(t)
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		doguVersionRegistryMock := NewMockDoguVersionRegistry(t)
		localDoguRepo := NewMockLocalDoguRepo(t)
		warpMenuPath := t.TempDir()

		warpMenuConfig := config.Configuration{
			Sources: []config.Source{
				{
					Path: "externals",
					Type: "externals",
				},
			},
		}
		mockExpectGetWarpMenuConfig(t, clientMock, warpMenuConfig)

		globalConfig := config2.CreateGlobalConfig(config2.Entries{
			"externals/myentry": config2.Value(multiline(
				`DisplayName: Test`,
				`URL: "https://test.example.com"`,
				`Description: Daily Tech News`,
				`Category: News`,
			)),
		})
		globalConfigRepoMock.EXPECT().Get(mock.Anything).Return(globalConfig, nil)

		reconciler := NewWarpMenuReconciler(
			clientMock,
			globalConfigRepoMock,
			doguVersionRegistryMock,
			localDoguRepo,
			warpMenuPath,
		)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "aNamespace", Name: "aConfigMap"}}
		_, err := reconciler.Reconcile(context.Background(), request)
		require.NoError(t, err)

		warpMenuCategories := parseWarpMenuCategoriesFromJsonFile(t, warpMenuPath)

		assert.Equal(t, 1, len(warpMenuCategories))
		assert.Equal(t, "News", warpMenuCategories[0].Title)

		expectedWarpMenuEntries := []testutils.WarpMenuEntry{
			{
				Title:       "Daily Tech News",
				DisplayName: "Test",
				Href:        "https://test.example.com",
				Target:      "external",
			},
		}
		assert.ElementsMatch(t, expectedWarpMenuEntries, warpMenuCategories[0].Entries)
	})

	t.Run("should create menu entries under category 'support' configured in the warp menu config map", func(t *testing.T) {
		clientMock := newMockK8sClient(t)
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		doguVersionRegistryMock := NewMockDoguVersionRegistry(t)
		localDoguRepo := NewMockLocalDoguRepo(t)
		warpMenuPath := t.TempDir()

		warpMenuConfig := config.Configuration{
			Support: []config.SupportSource{
				{
					Identifier: "id1",
					External:   true,
					Href:       "https://test.example.com",
				},
				{
					Identifier: "id2",
					External:   false,
					Href:       "/example",
				},
			},
		}
		mockExpectGetWarpMenuConfig(t, clientMock, warpMenuConfig)

		globalConfig := config2.CreateGlobalConfig(config2.Entries{})
		globalConfigRepoMock.EXPECT().Get(mock.Anything).Return(globalConfig, nil)

		reconciler := NewWarpMenuReconciler(
			clientMock,
			globalConfigRepoMock,
			doguVersionRegistryMock,
			localDoguRepo,
			warpMenuPath,
		)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "aNamespace", Name: "aConfigMap"}}
		_, err := reconciler.Reconcile(context.Background(), request)
		require.NoError(t, err)

		warpMenuCategories := parseWarpMenuCategoriesFromJsonFile(t, warpMenuPath)
		assert.Equal(t, 1, len(warpMenuCategories))
		assert.Equal(t, "Support", warpMenuCategories[0].Title)

		expectedWarpMenuEntries := []testutils.WarpMenuEntry{
			{
				Title:       "id1",
				DisplayName: "",
				Href:        "https://test.example.com",
				Target:      "external",
			},
			{
				Title:       "id2",
				DisplayName: "",
				Href:        "/example",
				Target:      "self",
			},
		}
		assert.ElementsMatch(t, expectedWarpMenuEntries, warpMenuCategories[0].Entries)
	})

	t.Run("should create menu entries for dogus", func(t *testing.T) {
		clientMock := newMockK8sClient(t)
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		doguVersionRegistryMock := NewMockDoguVersionRegistry(t)
		localDoguRepo := NewMockLocalDoguRepo(t)
		warpMenuPath := t.TempDir()

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

		reconciler := NewWarpMenuReconciler(
			clientMock,
			globalConfigRepoMock,
			doguVersionRegistryMock,
			localDoguRepo,
			warpMenuPath,
		)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "aNamespace", Name: "aConfigMap"}}
		_, err := reconciler.Reconcile(context.Background(), request)
		require.NoError(t, err)

		warpMenuCategories := parseWarpMenuCategoriesFromJsonFile(t, warpMenuPath)
		assert.Equal(t, 2, len(warpMenuCategories))

		devAppsWarpMenuCategory, found := findCategoryByTitle(warpMenuCategories, "DevApps")
		assert.True(t, found)
		devAppsExpectedWarpMenuEntries := []testutils.WarpMenuEntry{
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
		adminExpectedWarpMenuEntries := []testutils.WarpMenuEntry{
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

		reconciler := NewWarpMenuReconciler(
			clientMock,
			globalConfigRepoMock,
			doguVersionRegistryMock,
			localDoguRepo,
			warpMenuPath,
		)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "aNamespace", Name: "aConfigMap"}}
		_, err := reconciler.Reconcile(context.Background(), request)
		require.NoError(t, err)

		warpMenuCategories := parseWarpMenuCategoriesFromJsonFile(t, warpMenuPath)
		assert.Equal(t, 0, len(warpMenuCategories))
	})

	t.Run("should not create entries for the support category if this category is blocked", func(t *testing.T) {
		clientMock := newMockK8sClient(t)
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		doguVersionRegistryMock := NewMockDoguVersionRegistry(t)
		localDoguRepo := NewMockLocalDoguRepo(t)
		warpMenuPath := t.TempDir()

		warpMenuConfig := config.Configuration{
			Support: []config.SupportSource{
				{
					Identifier: "id1",
					External:   true,
					Href:       "https://test.example.com",
				},
				{
					Identifier: "id2",
					External:   false,
					Href:       "/example",
				},
			},
		}
		mockExpectGetWarpMenuConfig(t, clientMock, warpMenuConfig)

		globalConfig := config2.CreateGlobalConfig(config2.Entries{
			GlobalBlockWarpSupportCategoryConfigurationKey: config2.Value("true"),
		})
		globalConfigRepoMock.EXPECT().Get(mock.Anything).Return(globalConfig, nil)

		reconciler := NewWarpMenuReconciler(
			clientMock,
			globalConfigRepoMock,
			doguVersionRegistryMock,
			localDoguRepo,
			warpMenuPath,
		)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "aNamespace", Name: "aConfigMap"}}
		_, err := reconciler.Reconcile(context.Background(), request)
		require.NoError(t, err)

		warpMenuCategories := parseWarpMenuCategoriesFromJsonFile(t, warpMenuPath)
		assert.Equal(t, 0, len(warpMenuCategories))
	})

	t.Run("should create entries for the support category if it's blocked but some entries are allowed", func(t *testing.T) {
		clientMock := newMockK8sClient(t)
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		doguVersionRegistryMock := NewMockDoguVersionRegistry(t)
		localDoguRepo := NewMockLocalDoguRepo(t)
		warpMenuPath := t.TempDir()

		warpMenuConfig := config.Configuration{
			Support: []config.SupportSource{
				{
					Identifier: "id1",
					External:   true,
					Href:       "https://test.example.com",
				},
				{
					Identifier: "id2",
					External:   false,
					Href:       "/example",
				},
				{
					Identifier: "id3",
					External:   false,
					Href:       "/test",
				},
			},
		}
		mockExpectGetWarpMenuConfig(t, clientMock, warpMenuConfig)

		globalConfig := config2.CreateGlobalConfig(config2.Entries{
			GlobalBlockWarpSupportCategoryConfigurationKey:  "true",
			GlobalAllowedWarpSupportEntriesConfigurationKey: `["id1", "id2"]`,
		})
		globalConfigRepoMock.EXPECT().Get(mock.Anything).Return(globalConfig, nil)

		reconciler := NewWarpMenuReconciler(
			clientMock,
			globalConfigRepoMock,
			doguVersionRegistryMock,
			localDoguRepo,
			warpMenuPath,
		)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "aNamespace", Name: "aConfigMap"}}
		_, err := reconciler.Reconcile(context.Background(), request)
		require.NoError(t, err)

		warpMenuCategories := parseWarpMenuCategoriesFromJsonFile(t, warpMenuPath)
		assert.Equal(t, 1, len(warpMenuCategories))
		assert.Equal(t, "Support", warpMenuCategories[0].Title)

		expectedWarpMenuEntries := []testutils.WarpMenuEntry{
			{
				Title:       "id1",
				DisplayName: "",
				Href:        "https://test.example.com",
				Target:      "external",
			},
			{
				Title:       "id2",
				DisplayName: "",
				Href:        "/example",
				Target:      "self",
			},
		}
		assert.ElementsMatch(t, expectedWarpMenuEntries, warpMenuCategories[0].Entries)
	})

	t.Run("should not create entries for the support category if they are disabled", func(t *testing.T) {
		clientMock := newMockK8sClient(t)
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		doguVersionRegistryMock := NewMockDoguVersionRegistry(t)
		localDoguRepo := NewMockLocalDoguRepo(t)
		warpMenuPath := t.TempDir()

		warpMenuConfig := config.Configuration{
			Support: []config.SupportSource{
				{
					Identifier: "id1",
					External:   true,
					Href:       "https://test.example.com",
				},
				{
					Identifier: "id2",
					External:   false,
					Href:       "/example",
				},
				{
					Identifier: "id3",
					External:   false,
					Href:       "/test",
				},
			},
		}
		mockExpectGetWarpMenuConfig(t, clientMock, warpMenuConfig)

		globalConfig := config2.CreateGlobalConfig(config2.Entries{
			GlobalDisabledWarpSupportEntriesConfigurationKey: `["id1", "id3"]`,
		})
		globalConfigRepoMock.EXPECT().Get(mock.Anything).Return(globalConfig, nil)

		reconciler := NewWarpMenuReconciler(
			clientMock,
			globalConfigRepoMock,
			doguVersionRegistryMock,
			localDoguRepo,
			warpMenuPath,
		)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "aNamespace", Name: "aConfigMap"}}
		_, err := reconciler.Reconcile(context.Background(), request)
		require.NoError(t, err)

		warpMenuCategories := parseWarpMenuCategoriesFromJsonFile(t, warpMenuPath)
		assert.Equal(t, 1, len(warpMenuCategories))
		assert.Equal(t, "Support", warpMenuCategories[0].Title)

		expectedWarpMenuEntries := []testutils.WarpMenuEntry{
			{
				Title:       "id2",
				DisplayName: "",
				Href:        "/example",
				Target:      "self",
			},
		}
		assert.ElementsMatch(t, expectedWarpMenuEntries, warpMenuCategories[0].Entries)
	})

}

func newConfigMapWithName(name string) *v1.ConfigMap {
	return &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "aNamespace",
		},
	}
}

func multiline(parts ...string) string {
	return strings.Join(parts, "\n")
}

func parseWarpMenuCategoriesFromJsonFile(t *testing.T, warpMenuPath string) []testutils.WarpMenuCategory {
	data, err := os.ReadFile(warpMenuPath + "/menu.json")
	require.NoError(t, err)

	warpMenuCategories := &[]testutils.WarpMenuCategory{}
	err = json.Unmarshal(data, warpMenuCategories)
	require.NoError(t, err)

	return *warpMenuCategories
}

func mockExpectGetWarpMenuConfig(t *testing.T, clientMock *mockK8sClient, warpMenuConfig config.Configuration) {
	clientMock.EXPECT().
		Get(mock.Anything, mock.Anything, mock.Anything).
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

func findCategoryByTitle(warpMenuCategories []testutils.WarpMenuCategory, title string) (testutils.WarpMenuCategory, bool) {
	for _, cat := range warpMenuCategories {
		if cat.Title == title {
			return cat, true
		}
	}
	return testutils.WarpMenuCategory{}, false
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
