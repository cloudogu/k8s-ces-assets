package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

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
		t.Helper()
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

		globalConfig := config2.CreateGlobalConfig(config2.Entries{
			"externals/myentry": config2.Value(multiline(
				`DisplayName: Test`,
				`URL: "https://www.heise.de"`,
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

		data, err := os.ReadFile(warpMenuPath + "/menu.json")
		require.NoError(t, err)

		warpMenu, err := warpMenuCategoriesForm(data)
		require.NoError(t, err)

		assert.Equal(t, 1, len(warpMenu))
		assert.Equal(t, "News", warpMenu[0].Title)
		assert.Equal(t, 1, len(warpMenu[0].Entries))
		assert.Equal(t, "Daily Tech News", warpMenu[0].Entries[0].Title)
		assert.Equal(t, "Test", warpMenu[0].Entries[0].DisplayName)
		assert.Equal(t, "https://www.heise.de", warpMenu[0].Entries[0].Href)

	})

	t.Run("should create menu entries under category 'support' configured in the warp menu config map", func(t *testing.T) {
		t.Skip("TODO")
	})

	t.Run("should create menu entries for dogus under category 'applications' ", func(t *testing.T) {
		t.Skip("TODO")
	})

	t.Run("should not create entries for the support category if this category is blocked", func(t *testing.T) {
		t.Skip("TODO")
	})

	t.Run("should create entries for the support category if this category is block but entries are allowed", func(t *testing.T) {
		t.Skip("TODO")
	})

	t.Run("should not create entries for the support category if they are disabled ", func(t *testing.T) {
		t.Skip("TODO")
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

func warpMenuCategoriesForm(data []byte) ([]testutils.WarpMenuCategory, error) {
	warpMenuCategories := &[]testutils.WarpMenuCategory{}
	err := json.Unmarshal(data, warpMenuCategories)
	if err != nil {
		return []testutils.WarpMenuCategory{}, err
	}
	return *warpMenuCategories, nil
}
