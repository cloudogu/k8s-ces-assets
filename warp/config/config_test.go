package config

import (
	"context"
	_ "embed"
	"os"
	"testing"

	types2 "github.com/cloudogu/warp-assets/controller/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// Fixtures are embedded so tests are self-contained and do not depend on the working
// directory at runtime.

//go:embed testdata/k8s_config.yaml
var validConfigMapYAML []byte

//go:embed testdata/invalid_k8s_config.yaml
var invalidConfigMapYAML []byte

// makeConfigMap builds a ConfigMap whose warp.yaml data key holds the given YAML string.
func makeConfigMap(warpYAML string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WarpConfigMap,
			Namespace: "test",
		},
		Data: map[string]string{"warp.yaml": warpYAML},
	}
}

// fixtureConfigMap unmarshals a full ConfigMap from raw YAML bytes (used for the
// embedded fixtures that already contain the complete ConfigMap structure).
func fixtureConfigMap(t *testing.T, raw []byte) *corev1.ConfigMap {
	t.Helper()
	cm := &corev1.ConfigMap{}
	require.NoError(t, k8syaml.Unmarshal(raw, cm))
	return cm
}

// ── ReadConfiguration ────────────────────────────────────────────────────────

func TestReadConfiguration(t *testing.T) {
	const namespace = "test"
	ctx := context.Background()

	tests := []struct {
		name        string
		setupClient func() *corev1.ConfigMap // nil → no ConfigMap in cluster
		wantErr     string                   // non-empty → expect hard error containing this string
		wantLogErr  bool                     // expect config.LogErr to be non-nil
		check       func(t *testing.T, cfg *Configuration)
	}{
		{
			name: "valid config returns categories and entries",
			setupClient: func() *corev1.ConfigMap {
				return fixtureConfigMap(t, validConfigMapYAML)
			},
			check: func(t *testing.T, cfg *Configuration) {
				assert.Len(t, cfg.DefaultCategories, 2, "expected two categories from fixture")
				assert.NoError(t, cfg.LogErr, "no validation errors expected for valid config")
			},
		},
		{
			name:        "missing ConfigMap returns error",
			setupClient: func() *corev1.ConfigMap { return nil },
			wantErr:     "failed to get warp menu configmap",
		},
		{
			name: "invalid YAML in warp.yaml returns error",
			setupClient: func() *corev1.ConfigMap {
				return fixtureConfigMap(t, invalidConfigMapYAML)
			},
			wantErr: "failed to unmarshal yaml from warp config",
		},
		{
			name: "empty categories section is accepted",
			setupClient: func() *corev1.ConfigMap {
				return makeConfigMap("categories: {}\ndefaultEntries: {}\n")
			},
			check: func(t *testing.T, cfg *Configuration) {
				assert.Empty(t, cfg.DefaultCategories, "no categories expected")
			},
		},
		{
			name: "empty defaultEntries section is accepted",
			setupClient: func() *corev1.ConfigMap {
				return makeConfigMap(`
categories:
  Support:
    order: 400
    displayName:
      de: "Support"
      en: "Support"
defaultEntries: {}
`)
			},
			check: func(t *testing.T, cfg *Configuration) {
				assert.Len(t, cfg.DefaultCategories, 1, "one category expected")
			},
		},
		{
			name: "unknown locale in displayName is logged and falls back to identifier",
			setupClient: func() *corev1.ConfigMap {
				return makeConfigMap(`
categories: {}
defaultEntries:
  myEntry:
    category: "support"
    displayName:
      fr: "Bonjour"
    href: "/some/path"
`)
			},
			wantLogErr: true,
			check: func(t *testing.T, cfg *Configuration) {
				require.Len(t, cfg.DefaultCategories, 1, "entry must still be created despite unknown locale")
				require.Len(t, cfg.DefaultCategories[0].Entries, 1)
				entry := cfg.DefaultCategories[0].Entries[0]
				assert.Equal(t, "myEntry", entry.DisplayName[types2.LocaleDe],
					"de display name should fall back to identifier")
				assert.Equal(t, "myEntry", entry.DisplayName[types2.LocaleEn],
					"en display name should fall back to identifier")
			},
		},
		{
			name: "external URL sets TARGET_EXTERNAL",
			setupClient: func() *corev1.ConfigMap {
				return makeConfigMap(`
categories: {}
defaultEntries:
  externalLink:
    category: "support"
    displayName:
      de: "Extern"
      en: "External"
    href: "https://docs.cloudogu.com"
`)
			},
			check: func(t *testing.T, cfg *Configuration) {
				require.Len(t, cfg.DefaultCategories, 1)
				require.Len(t, cfg.DefaultCategories[0].Entries, 1)
				assert.Equal(t, types2.TARGET_EXTERNAL, cfg.DefaultCategories[0].Entries[0].Target,
					"absolute URL must produce TARGET_EXTERNAL")
			},
		},
		{
			name: "relative URL sets TARGET_SELF",
			setupClient: func() *corev1.ConfigMap {
				return makeConfigMap(`
categories: {}
defaultEntries:
  internalLink:
    category: "support"
    displayName:
      de: "Intern"
      en: "Internal"
    href: "/info/about"
`)
			},
			check: func(t *testing.T, cfg *Configuration) {
				require.Len(t, cfg.DefaultCategories, 1)
				require.Len(t, cfg.DefaultCategories[0].Entries, 1)
				assert.Equal(t, types2.TARGET_SELF, cfg.DefaultCategories[0].Entries[0].Target,
					"relative URL must produce TARGET_SELF")
			},
		},
		{
			name: "unknown locale in category displayName is logged and falls back to identifier",
			setupClient: func() *corev1.ConfigMap {
				return makeConfigMap(`
categories:
  myCategory:
    displayName:
      fr: "Inconnu"
defaultEntries: {}
`)
			},
			wantLogErr: true,
			check: func(t *testing.T, cfg *Configuration) {
				require.Len(t, cfg.DefaultCategories, 1, "category must still be created despite unknown locale")
				cat := cfg.DefaultCategories[0]
				assert.Equal(t, "myCategory", cat.DisplayName[types2.LocaleDe],
					"de display name should fall back to identifier")
				assert.Equal(t, "myCategory", cat.DisplayName[types2.LocaleEn],
					"en display name should fall back to identifier")
			},
		},
		{
			name: "category without explicit order defaults to 9999",
			setupClient: func() *corev1.ConfigMap {
				return makeConfigMap(`
categories:
  NoOrder:
    displayName:
      de: "Kein"
      en: "None"
defaultEntries: {}
`)
			},
			check: func(t *testing.T, cfg *Configuration) {
				require.Len(t, cfg.DefaultCategories, 1)
				assert.Equal(t, 9999, cfg.DefaultCategories[0].Order,
					"missing order must default to 9999")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := fake.NewClientBuilder()
			if cm := tt.setupClient(); cm != nil {
				builder = builder.WithObjects(cm)
			}
			fakeClient := builder.Build()

			cfg, err := ReadConfiguration(ctx, fakeClient, namespace)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)

			if tt.wantLogErr {
				assert.Error(t, cfg.LogErr, "expected a non-nil LogErr for validation issues")
			}

			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

// ── WarpConfigMapPredicate ───────────────────────────────────────────────────

func TestWarpConfigMapPredicate(t *testing.T) {
	p := WarpConfigMapPredicate()

	warpCM := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: WarpConfigMap}}
	otherCM := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "other-config"}}

	t.Run("Create passes only for the warp ConfigMap", func(t *testing.T) {
		assert.True(t, p.Create(event.CreateEvent{Object: warpCM}))
		assert.False(t, p.Create(event.CreateEvent{Object: otherCM}))
	})

	t.Run("Delete passes only for the warp ConfigMap", func(t *testing.T) {
		assert.True(t, p.Delete(event.DeleteEvent{Object: warpCM}))
		assert.False(t, p.Delete(event.DeleteEvent{Object: otherCM}))
	})

	t.Run("Generic passes only for the warp ConfigMap", func(t *testing.T) {
		assert.True(t, p.Generic(event.GenericEvent{Object: warpCM}))
		assert.False(t, p.Generic(event.GenericEvent{Object: otherCM}))
	})

	t.Run("Update passes only for the warp ConfigMap name", func(t *testing.T) {
		assert.False(t, p.Update(event.UpdateEvent{ObjectOld: otherCM, ObjectNew: otherCM}))
	})

	t.Run("Update returns false when Data and BinaryData are unchanged", func(t *testing.T) {
		old := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: WarpConfigMap},
			Data:       map[string]string{"warp.yaml": "foo"},
		}
		assert.False(t, p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: old.DeepCopy()}))
	})

	t.Run("Update returns true when Data changes", func(t *testing.T) {
		old := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: WarpConfigMap},
			Data:       map[string]string{"warp.yaml": "foo"},
		}
		newCM := old.DeepCopy()
		newCM.Data["warp.yaml"] = "bar"
		assert.True(t, p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: newCM}))
	})

	t.Run("Update returns true when BinaryData changes", func(t *testing.T) {
		old := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: WarpConfigMap},
			BinaryData: map[string][]byte{"k": {1}},
		}
		newCM := old.DeepCopy()
		newCM.BinaryData["k"] = []byte{2}
		assert.True(t, p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: newCM}))
	})

	t.Run("Update returns false when objects are not ConfigMaps", func(t *testing.T) {
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: WarpConfigMap}}
		assert.False(t, p.Update(event.UpdateEvent{ObjectOld: secret, ObjectNew: secret}))
	})
}

// ── ReadWatchNamespace ───────────────────────────────────────────────────────

func TestReadWatchNamespace(t *testing.T) {
	tests := []struct {
		name      string
		envValue  string
		setEnv    bool
		wantValue string
		wantErr   string
	}{
		{
			name:      "environment variable is set returns the value without error",
			setEnv:    true,
			envValue:  "my-namespace",
			wantValue: "my-namespace",
		},
		{
			name:    "environment variable is unset returns a descriptive error",
			setEnv:  false,
			wantErr: "failed to read namespace to watch from environment variable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous, exists := os.LookupEnv(namespaceEnvVar)
			defer func() {
				if exists {
					os.Setenv(namespaceEnvVar, previous)
				} else {
					os.Unsetenv(namespaceEnvVar)
				}
			}()

			if tt.setEnv {
				require.NoError(t, os.Setenv(namespaceEnvVar, tt.envValue))
			} else {
				require.NoError(t, os.Unsetenv(namespaceEnvVar))
			}

			got, err := ReadWatchNamespace()

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantValue, got)
		})
	}
}

// ── ReadWarpPath ─────────────────────────────────────────────────────────────

func TestReadWarpPath(t *testing.T) {
	tests := []struct {
		name      string
		setEnv    bool
		envValue  string
		wantValue string
		wantErr   string
	}{
		{
			name:      "environment variable is set returns the value without error",
			setEnv:    true,
			envValue:  "/var/www/html/warp/menu",
			wantValue: "/var/www/html/warp/menu",
		},
		{
			name:    "environment variable is unset returns a descriptive error",
			setEnv:  false,
			wantErr: "failed to read warp path to watch from environment variable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous, exists := os.LookupEnv(warpPathEnvVar)
			defer func() {
				if exists {
					os.Setenv(warpPathEnvVar, previous)
				} else {
					os.Unsetenv(warpPathEnvVar)
				}
			}()

			if tt.setEnv {
				require.NoError(t, os.Setenv(warpPathEnvVar, tt.envValue))
			} else {
				require.NoError(t, os.Unsetenv(warpPathEnvVar))
			}

			got, err := ReadWarpPath()

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantValue, got)
		})
	}
}

// ── ReadComponentName ───────────────────────────────────────────────────────

func TestReadComponentName(t *testing.T) {
	tests := []struct {
		name      string
		setEnv    bool
		envValue  string
		wantValue string
		wantErr   string
	}{
		{
			name:      "environment variable is set returns the value without error",
			setEnv:    true,
			envValue:  "k8s-ces-assets",
			wantValue: "k8s-ces-assets",
		},
		{
			name:    "environment variable is unset returns a descriptive error",
			setEnv:  false,
			wantErr: "failed to read component name from environment variable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous, exists := os.LookupEnv(componentNameEnvVar)
			defer func() {
				if exists {
					os.Setenv(componentNameEnvVar, previous)
				} else {
					os.Unsetenv(componentNameEnvVar)
				}
			}()

			if tt.setEnv {
				require.NoError(t, os.Setenv(componentNameEnvVar, tt.envValue))
			} else {
				require.NoError(t, os.Unsetenv(componentNameEnvVar))
			}

			got, err := ReadComponentName()

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantValue, got)
		})
	}
}
