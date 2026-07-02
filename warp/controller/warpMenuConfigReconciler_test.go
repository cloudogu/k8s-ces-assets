package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	component "github.com/cloudogu/k8s-component-lib/api/v1"
	warpmenu "github.com/cloudogu/k8s-warp-menu-entry-lib/api/v1"
	"github.com/cloudogu/warp-assets/config"
	domain "github.com/cloudogu/warp-assets/controller/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	testDeploymentName = "aDeployment"
	testNamespace      = "aNamespace"
)

var testComponentCRKey = types.NamespacedName{Name: testDeploymentName, Namespace: testNamespace}

// ── Reconcile ────────────────────────────────────────────────────────────────

func TestReconcile(t *testing.T) {
	t.Run("two valid entries are placed in their respective categories", func(t *testing.T) {
		firstEntry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		secondEntry := buildWarpMenuEntry("dogu2", "Admin", "/dogu_2", "Dogu 2", "Dogu 2 en", false)
		clientMock := getClientMock(t, []warpmenu.WarpMenuEntry{firstEntry, secondEntry})
		recorder := events.NewFakeRecorder(5)
		reconciler := NewWarpMenuReconciler(clientMock, recorder, t.TempDir(), testComponentCRKey)

		req := reqFor(firstEntry)
		_, err := reconciler.Reconcile(context.Background(), req)
		require.NoError(t, err)

		info, err := os.Stat(reconciler.warpMenuPath + "/menu.json")
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0644), info.Mode().Perm())

		categories := parseMenuJSON(t, reconciler.warpMenuPath)
		assert.Len(t, categories, 2)
		devApps, found := findCategory(categories, "DevApps")
		assert.True(t, found)
		assert.ElementsMatch(t, expectedEntry("dogu1", "Dogu 1", "Dogu 1 en", "/dogu_1"), devApps.Entries)
		admin, found := findCategory(categories, "Admin")
		assert.True(t, found)
		assert.ElementsMatch(t, expectedEntry("dogu2", "Dogu 2", "Dogu 2 en", "/dogu_2"), admin.Entries)

		// only the reconciled entry gets its status updated
		verifyReadyStatus(t, clientMock, req, false)
		verifyNoStatus(t, clientMock, &secondEntry)
		assertEvent(t, recorder, reasonMenuUpdated, "warp menu entries have been updated.")
	})

	t.Run("disabled entry is absent from menu but gets ConditionVisible=False", func(t *testing.T) {
		entry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", true)
		clientMock := getClientMock(t, []warpmenu.WarpMenuEntry{entry})
		recorder := events.NewFakeRecorder(5)
		reconciler := NewWarpMenuReconciler(clientMock, recorder, t.TempDir(), testComponentCRKey)

		req := reqFor(entry)
		_, err := reconciler.Reconcile(context.Background(), req)
		require.NoError(t, err)

		categories := parseMenuJSON(t, reconciler.warpMenuPath)
		assert.Empty(t, categories)
		verifyReadyStatus(t, clientMock, req, true /* disabled */)
		assertEvent(t, recorder, reasonMenuUpdated, "warp menu entries have been updated.")
	})

	t.Run("reconciling an entry whose status is already set is idempotent", func(t *testing.T) {
		entry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		setReadyStatus(&entry)
		clientMock := getClientMock(t, []warpmenu.WarpMenuEntry{entry})
		recorder := events.NewFakeRecorder(5)
		reconciler := NewWarpMenuReconciler(clientMock, recorder, t.TempDir(), testComponentCRKey)

		req := reqFor(entry)
		_, err := reconciler.Reconcile(context.Background(), req)
		require.NoError(t, err)

		categories := parseMenuJSON(t, reconciler.warpMenuPath)
		assert.Len(t, categories, 1)
		verifyReadyStatus(t, clientMock, req, false)
	})

	t.Run("reconcile triggered by a deleted entry rebuilds the menu from remaining entries", func(t *testing.T) {
		firstEntry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		secondEntry := buildWarpMenuEntry("dogu2", "Admin", "/dogu_2", "Dogu 2", "Dogu 2 en", false)
		clientMock := getClientMock(t, []warpmenu.WarpMenuEntry{firstEntry, secondEntry})
		recorder := events.NewFakeRecorder(5)
		reconciler := NewWarpMenuReconciler(clientMock, recorder, t.TempDir(), testComponentCRKey)

		// Reconcile is triggered for "deletedEntry" which does not exist
		req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "deletedEntry"}}
		_, err := reconciler.Reconcile(context.Background(), req)
		require.NoError(t, err)

		categories := parseMenuJSON(t, reconciler.warpMenuPath)
		assert.Len(t, categories, 2)
		assertEvent(t, recorder, reasonMenuUpdated, "warp menu entries have been updated.")
	})

	t.Run("entry with empty category is skipped and gets ConditionReady=False", func(t *testing.T) {
		invalidEntry := buildWarpMenuEntry("dogu1", "" /* empty category */, "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		clientMock := getClientMock(t, []warpmenu.WarpMenuEntry{invalidEntry})
		recorder := events.NewFakeRecorder(5)
		reconciler := NewWarpMenuReconciler(clientMock, recorder, t.TempDir(), testComponentCRKey)

		req := reqFor(invalidEntry)
		_, err := reconciler.Reconcile(context.Background(), req)
		require.NoError(t, err, "invalid entry must not fail the reconcile")

		categories := parseMenuJSON(t, reconciler.warpMenuPath)
		assert.Empty(t, categories, "invalid entry must not appear in the menu")
		verifyInvalidEntryStatus(t, clientMock, req)
	})

	t.Run("valid entries are rendered while invalid ones are skipped; reconcile succeeds", func(t *testing.T) {
		validEntry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		invalidEntry := buildWarpMenuEntry("dogu2", "" /* empty category */, "/dogu_2", "Dogu 2", "Dogu 2 en", false)
		clientMock := getClientMock(t, []warpmenu.WarpMenuEntry{validEntry, invalidEntry})
		recorder := events.NewFakeRecorder(10)
		reconciler := NewWarpMenuReconciler(clientMock, recorder, t.TempDir(), testComponentCRKey)

		req := reqFor(validEntry)
		_, err := reconciler.Reconcile(context.Background(), req)
		require.NoError(t, err)

		categories := parseMenuJSON(t, reconciler.warpMenuPath)
		assert.Len(t, categories, 1)
		devApps, found := findCategory(categories, "DevApps")
		assert.True(t, found)
		assert.ElementsMatch(t, expectedEntry("dogu1", "Dogu 1", "Dogu 1 en", "/dogu_1"), devApps.Entries)

		verifyReadyStatus(t, clientMock, req, false)
	})

	t.Run("ConfigMap-triggered reconcile rebuilds the menu without updating any CR status", func(t *testing.T) {
		entry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		clientMock := getClientMock(t, []warpmenu.WarpMenuEntry{entry})
		recorder := events.NewFakeRecorder(5)
		reconciler := NewWarpMenuReconciler(clientMock, recorder, t.TempDir(), testComponentCRKey)

		// ConfigMap change produces a request with Name = config.WarpConfigMap
		req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: config.WarpConfigMap}}
		_, err := reconciler.Reconcile(context.Background(), req)
		require.NoError(t, err)

		categories := parseMenuJSON(t, reconciler.warpMenuPath)
		assert.Len(t, categories, 1)

		// The entry CR must have no status conditions written
		verifyNoStatus(t, clientMock, &entry)
	})

	t.Run("listing WarpMenuEntry CRs fails — error returned and entry gets ConditionReady=False", func(t *testing.T) {
		entry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		clientMock := getClientMockWithListError(t, []warpmenu.WarpMenuEntry{entry})
		recorder := events.NewFakeRecorder(5)
		reconciler := NewWarpMenuReconciler(clientMock, recorder, t.TempDir(), testComponentCRKey)

		req := reqFor(entry)
		_, err := reconciler.Reconcile(context.Background(), req)
		assert.ErrorContains(t, err, "failed to list warp menu entry CRs")
		assert.ErrorContains(t, err, "Simulating api error")
		verifyInternalErrorStatus(t, clientMock, req)
		assertEvent(t, recorder, reasonFailedListEntries, "failed to list warp menu entry CRs")
	})

	t.Run("warp ConfigMap is missing — error returned and entry gets ConditionReady=False", func(t *testing.T) {
		entry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		clientMock := getClientMockWithoutConfigMap(t, []warpmenu.WarpMenuEntry{entry})
		recorder := events.NewFakeRecorder(5)
		reconciler := NewWarpMenuReconciler(clientMock, recorder, t.TempDir(), testComponentCRKey)

		req := reqFor(entry)
		_, err := reconciler.Reconcile(context.Background(), req)
		assert.ErrorContains(t, err, "failed to read warp configuration")
		assert.ErrorContains(t, err, config.WarpConfigMap)
		verifyInternalErrorStatus(t, clientMock, req)
		assertEvent(t, recorder, reasonFailedReadConfig, "failed to read warp configuration")
	})

	t.Run("writing the warp menu file fails — error returned and entry gets ConditionReady=False", func(t *testing.T) {
		entry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		clientMock := getClientMock(t, []warpmenu.WarpMenuEntry{entry})
		recorder := events.NewFakeRecorder(5)
		reconciler := NewWarpMenuReconciler(clientMock, recorder, "nonexistingpath", testComponentCRKey)

		req := reqFor(entry)
		_, err := reconciler.Reconcile(context.Background(), req)
		assert.ErrorContains(t, err, "failed to write warp menu file")
		verifyInternalErrorStatus(t, clientMock, req)
		assertEvent(t, recorder, reasonFailedWriteMenu, "failed to write warp menu file")
	})

	t.Run("status subresource update fails — error returned", func(t *testing.T) {
		entry := buildWarpMenuEntry("dogu1", "DevApps", "/dogu_1", "Dogu 1", "Dogu 1 en", false)
		clientMock := getClientMockWithStatusUpdateError(t, []warpmenu.WarpMenuEntry{entry})
		recorder := events.NewFakeRecorder(5)
		reconciler := NewWarpMenuReconciler(clientMock, recorder, t.TempDir(), testComponentCRKey)

		req := reqFor(entry)
		_, err := reconciler.Reconcile(context.Background(), req)
		assert.ErrorContains(t, err, "failed to update status conditions for entry")
		assert.ErrorContains(t, err, "mocked SubResourceClient error")
	})
}

// ── ConfigMap watch mapper ────────────────────────────────────────────────────

func TestWarpConfigChangeReconcile(t *testing.T) {
	t.Run("ConfigMap change produces a reconcile request with the WarpConfigMap sentinel name", func(t *testing.T) {
		cm := &warpmenu.WarpMenuEntry{} // any client.Object with a namespace is fine
		cm.Name = config.WarpConfigMap
		cm.Namespace = testNamespace

		requests := warpConfigChangeReconcile(context.Background(), cm)

		expected := []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Name:      config.WarpConfigMap,
				Namespace: testNamespace,
			},
		}}
		assert.Equal(t, expected, requests)
	})
}

// ── validatePath ─────────────────────────────────────────────────────────────

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid server-relative path", "/my-app", false},
		{"valid path with query string", "/my-app?foo=bar", false},
		{"too short — single char", "/", true},
		{"too short — empty string", "", true},
		{"absolute URL rejected", "https://example.com/app", true},
		{"protocol-relative URL rejected", "//example.com/app", true},
		{"javascript scheme rejected", "javascript:alert(1)", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePath(tc.path)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ── mapWarpCRToEntryWithCategory ─────────────────────────────────────────────

func TestMapWarpCRToEntryWithCategory(t *testing.T) {
	t.Run("valid CR produces correct domain entry", func(t *testing.T) {
		cr := buildWarpMenuEntry("my-app", "DevApps", "/my-app", "Meine App", "My App", false)
		entry, err := mapWarpCRToEntryWithCategory(cr)

		require.NoError(t, err)
		assert.Equal(t, "DevApps", entry.Category)
		assert.Equal(t, "my-app", entry.Entry.Identifier)
		assert.Equal(t, "/my-app", entry.Entry.Href)
		assert.Equal(t, domain.TARGET_SELF, entry.Entry.Target)
		assert.Equal(t, "Meine App", entry.Entry.Localization[domain.LocaleDe])
		assert.Equal(t, "My App", entry.Entry.Localization[domain.LocaleEn])
	})

	t.Run("empty category returns error", func(t *testing.T) {
		cr := buildWarpMenuEntry("my-app", "", "/my-app", "Name", "Name", false)
		_, err := mapWarpCRToEntryWithCategory(cr)
		assert.ErrorContains(t, err, "category is empty")
	})

	t.Run("invalid path returns error", func(t *testing.T) {
		cr := buildWarpMenuEntry("my-app", "DevApps", "https://external.com", "Name", "Name", false)
		_, err := mapWarpCRToEntryWithCategory(cr)
		assert.ErrorContains(t, err, "invalid path")
	})

	t.Run("both category and path invalid — errors are joined", func(t *testing.T) {
		cr := buildWarpMenuEntry("my-app", "", "https://external.com", "Name", "Name", false)
		_, err := mapWarpCRToEntryWithCategory(cr)
		assert.ErrorContains(t, err, "category is empty")
		assert.ErrorContains(t, err, "invalid path")
	})
}

// ── Test helpers ──────────────────────────────────────────────────────────────

func reqFor(entry warpmenu.WarpMenuEntry) ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: entry.Name}}
}

func parseMenuJSON(t *testing.T, warpMenuPath string) []warpMenuCategoryJSON {
	t.Helper()
	data, err := os.ReadFile(warpMenuPath + "/menu.json")
	require.NoError(t, err)
	var categories []warpMenuCategoryJSON
	require.NoError(t, json.Unmarshal(data, &categories))
	return categories
}

func findCategory(categories []warpMenuCategoryJSON, title string) (warpMenuCategoryJSON, bool) {
	for _, c := range categories {
		if c.Title == title {
			return c, true
		}
	}
	return warpMenuCategoryJSON{}, false
}

func expectedEntry(name, displayNameDE, displayNameEN, href string) []warpMenuEntryJSON {
	return []warpMenuEntryJSON{{
		Title:       name,
		DisplayName: displayNameDE,
		Href:        href,
		Target:      "self",
		Localization: map[string]string{
			"de": displayNameDE,
			"en": displayNameEN,
		},
	}}
}

// verifyReadyStatus asserts that the WarpMenuEntry has ConditionReady=True and
// ConditionVisible reflecting the disabled flag.
func verifyReadyStatus(t *testing.T, c client.WithWatch, req ctrl.Request, disabled bool) {
	t.Helper()
	entry := &warpmenu.WarpMenuEntry{}
	require.NoError(t, c.Get(context.Background(), req.NamespacedName, entry))
	require.Len(t, entry.Status.Conditions, 2)

	readyCond := meta.FindStatusCondition(entry.Status.Conditions, warpmenu.ConditionReady)
	require.NotNil(t, readyCond)
	assert.Equal(t, metav1.ConditionTrue, readyCond.Status)
	assert.Equal(t, reasonMenuUpdated, readyCond.Reason)

	visibleCond := meta.FindStatusCondition(entry.Status.Conditions, warpmenu.ConditionVisible)
	require.NotNil(t, visibleCond)
	if disabled {
		assert.Equal(t, metav1.ConditionFalse, visibleCond.Status)
		assert.Equal(t, warpmenu.ReasonEntryHidden, visibleCond.Reason)
	} else {
		assert.Equal(t, metav1.ConditionTrue, visibleCond.Status)
		assert.Equal(t, warpmenu.ReasonEntryRendered, visibleCond.Reason)
	}
}

// verifyInvalidEntryStatus asserts ConditionReady=False and ConditionVisible=False
// with reasonInvalidEntry — written when the entry failed validation.
func verifyInvalidEntryStatus(t *testing.T, c client.WithWatch, req ctrl.Request) {
	t.Helper()
	entry := &warpmenu.WarpMenuEntry{}
	require.NoError(t, c.Get(context.Background(), req.NamespacedName, entry))
	require.Len(t, entry.Status.Conditions, 2)

	readyCond := meta.FindStatusCondition(entry.Status.Conditions, warpmenu.ConditionReady)
	require.NotNil(t, readyCond)
	assert.Equal(t, metav1.ConditionFalse, readyCond.Status)
	assert.Equal(t, reasonInvalidEntry, readyCond.Reason)

	visibleCond := meta.FindStatusCondition(entry.Status.Conditions, warpmenu.ConditionVisible)
	require.NotNil(t, visibleCond)
	assert.Equal(t, metav1.ConditionFalse, visibleCond.Status)
	assert.Equal(t, reasonInvalidEntry, visibleCond.Reason)
}

// verifyInternalErrorStatus asserts ConditionReady=False with the generic
// operator-error message — written when reconcile itself failed.
func verifyInternalErrorStatus(t *testing.T, c client.WithWatch, req ctrl.Request) {
	t.Helper()
	entry := &warpmenu.WarpMenuEntry{}
	require.NoError(t, c.Get(context.Background(), req.NamespacedName, entry))
	require.Len(t, entry.Status.Conditions, 1)

	readyCond := meta.FindStatusCondition(entry.Status.Conditions, warpmenu.ConditionReady)
	require.NotNil(t, readyCond)
	assert.Equal(t, metav1.ConditionFalse, readyCond.Status)
	assert.Equal(t, reasonMenuGenerationFailed, readyCond.Reason)
	assert.Equal(t, "an internal operator error occurred", readyCond.Message)
}

// verifyNoStatus asserts the entry has no status conditions set.
func verifyNoStatus(t *testing.T, c client.WithWatch, entry *warpmenu.WarpMenuEntry) {
	t.Helper()
	current := &warpmenu.WarpMenuEntry{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Namespace: testNamespace, Name: entry.Name}, current))
	assert.Empty(t, current.Status.Conditions)
}

// assertEvent reads one event from the recorder and checks it contains the given substrings.
func assertEvent(t *testing.T, recorder *events.FakeRecorder, substrings ...string) {
	t.Helper()
	select {
	case event := <-recorder.Events:
		for _, s := range substrings {
			assert.Contains(t, event, s)
		}
	default:
		t.Fatal("expected an event to be recorded but the channel was empty")
	}
}

// setReadyStatus pre-populates an entry with a Ready=True condition to test idempotency.
func setReadyStatus(entry *warpmenu.WarpMenuEntry) {
	meta.SetStatusCondition(&entry.Status.Conditions, metav1.Condition{
		Type:   warpmenu.ConditionReady,
		Status: metav1.ConditionTrue,
		Reason: warpmenu.ReasonEntryRendered,
	})
}

// ── JSON mirror types (for parsing menu.json in assertions) ──────────────────

// warpMenuCategoryJSON mirrors the JSON shape of a category entry in menu.json.
type warpMenuCategoryJSON struct {
	Title   string              `json:"Title"`
	Order   int                 `json:"Order"`
	Entries []warpMenuEntryJSON `json:"Entries"`
}

// warpMenuEntryJSON mirrors the JSON shape of a link entry in menu.json.
type warpMenuEntryJSON struct {
	Title        string            `json:"Title"`
	DisplayName  string            `json:"DisplayName"`
	Href         string            `json:"Href"`
	Target       string            `json:"Target"`
	Localization map[string]string `json:"Localization"`
}

// ── Fake client builders ─────────────────────────────────────────────────────

func buildWarpMenuEntry(name, category, path, displayNameDE, displayNameEN string, disabled bool) warpmenu.WarpMenuEntry {
	return warpmenu.WarpMenuEntry{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Spec: warpmenu.WarpMenuEntrySpec{
			Category: category,
			Path:     path,
			DisplayName: warpmenu.DisplayName{
				DE: displayNameDE,
				EN: displayNameEN,
			},
			Disabled: disabled,
		},
	}
}

func newClientBuilder(t *testing.T) *fake.ClientBuilder {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, warpmenu.AddToScheme(scheme))
	require.NoError(t, component.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme)
}

func buildComponentCR() *component.Component {
	return &component.Component{
		ObjectMeta: metav1.ObjectMeta{Name: testDeploymentName, Namespace: testNamespace},
	}
}

func getConfigMap(t *testing.T, warpMenuConfig config.Configuration) *corev1.ConfigMap {
	warpMenuConfigAsString, err := yaml.Marshal(warpMenuConfig)
	require.NoError(t, err)
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.WarpConfigMap, Namespace: testNamespace},
		Data:       map[string]string{"warp.yaml": string(warpMenuConfigAsString)},
	}
}

func getClientMock(t *testing.T, entries []warpmenu.WarpMenuEntry) client.WithWatch {
	return newClientBuilder(t).
		WithRuntimeObjects(
			buildComponentCR(),
			getConfigMap(t, config.Configuration{}),
			&warpmenu.WarpMenuEntryList{Items: entries},
		).
		WithStatusSubresource(getFirstWarpMenuEntry(entries)).
		Build()
}

func getClientMockWithoutConfigMap(t *testing.T, entries []warpmenu.WarpMenuEntry) client.WithWatch {
	return newClientBuilder(t).
		WithRuntimeObjects(
			buildComponentCR(),
			&warpmenu.WarpMenuEntryList{Items: entries},
		).
		WithStatusSubresource(getFirstWarpMenuEntry(entries)).
		Build()
}

func getClientMockWithListError(t *testing.T, entries []warpmenu.WarpMenuEntry) client.WithWatch {
	return newClientBuilder(t).
		WithRuntimeObjects(
			buildComponentCR(),
			getConfigMap(t, config.Configuration{}),
			&warpmenu.WarpMenuEntryList{Items: entries},
		).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, cl client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				return fmt.Errorf("Simulating api error")
			},
		}).
		WithStatusSubresource(getFirstWarpMenuEntry(entries)).
		Build()
}

func getClientMockWithStatusUpdateError(t *testing.T, entries []warpmenu.WarpMenuEntry) client.WithWatch {
	return newClientBuilder(t).
		WithRuntimeObjects(
			buildComponentCR(),
			getConfigMap(t, config.Configuration{}),
			&warpmenu.WarpMenuEntryList{Items: entries},
		).
		WithStatusSubresource(getFirstWarpMenuEntry(entries)).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResource: func(cl client.WithWatch, subResource string) client.SubResourceClient {
				if subResource == "status" {
					return &mockSubResourceClient{SubResourceClient: cl.SubResource(subResource)}
				}
				return cl.SubResource(subResource)
			},
		}).
		Build()
}

func getFirstWarpMenuEntry(entries []warpmenu.WarpMenuEntry) *warpmenu.WarpMenuEntry {
	if len(entries) > 0 {
		return &entries[0]
	}
	return &warpmenu.WarpMenuEntry{}
}

type mockSubResourceClient struct {
	client.SubResourceClient
}

func (m *mockSubResourceClient) Update(_ context.Context, _ client.Object, _ ...client.SubResourceUpdateOption) error {
	return fmt.Errorf("mocked SubResourceClient error")
}
