package config

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"reflect"

	types2 "github.com/cloudogu/warp-assets/controller/types"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	WarpConfigMap = "k8s-ces-warp-config"
	warpConfigKey = "warp.yaml"
	// namespaceEnvVar is the environment variable that defines the Kubernetes namespace
	// the controller should watch for WarpMenuEntry resources.
	namespaceEnvVar     = "WATCH_NAMESPACE"
	warpPathEnvVar      = "WARP_PATH"
	componentNameEnvVar = "COMPONENT_NAME"
)

var logger = ctrl.Log.WithName("k8s-ces-assets.config")

// DisplayNameDTO maps locale tags (e.g. "de", "en") to their translated strings as
// read from the warp ConfigMap YAML.
type DisplayNameDTO map[string]string

// CategoryDTO is the YAML representation of a warp menu category.
type CategoryDTO struct {
	Order       *int           `yaml:"order,omitempty"`
	DisplayName DisplayNameDTO `yaml:"displayName"`
}

// EntryDTO is the YAML representation of a default warp menu entry.
type EntryDTO struct {
	Category    string         `yaml:"category"`
	Enabled     bool           `yaml:"enabled"`
	DisplayName DisplayNameDTO `yaml:"displayName"`
	Link        string         `yaml:"href"`
}

// WarpYamlDTO is the top-level structure of the warp.yaml key in the ConfigMap.
type WarpYamlDTO struct {
	Categories     map[string]yaml.Node `yaml:"categories"`
	DefaultEntries map[string]yaml.Node `yaml:"defaultEntries"`
}

// Configuration holds the parsed warp menu configuration.
type Configuration struct {
	// LogErr captures non-fatal validation errors (e.g. unknown locales) that
	// should be logged but do not prevent the menu from being built.
	LogErr            error
	DefaultCategories types2.Categories
}

// ReadConfiguration fetches the warp menu ConfigMap from the cluster and parses it
// into a Configuration. Non-fatal validation errors (e.g. unknown locales) are
// returned via Configuration.LogErr rather than as a hard error.
func ReadConfiguration(ctx context.Context, client client.Client, namespace string) (*Configuration, error) {
	var validationErrs []error

	configmap := &corev1.ConfigMap{}
	objectKey := types.NamespacedName{
		Namespace: namespace,
		Name:      WarpConfigMap,
	}

	if err := client.Get(ctx, objectKey, configmap); err != nil {
		return nil, fmt.Errorf("failed to get warp menu configmap: %w", err)
	}

	data, ok := configmap.Data[warpConfigKey]
	if !ok {
		return nil, fmt.Errorf("warp config %q is missing required key %q", WarpConfigMap, warpConfigKey)
	}

	var yamlData WarpYamlDTO

	if err := yaml.Unmarshal([]byte(data), &yamlData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml from warp config: %w", err)
	}

	defaultCategories, err := mapYamlConfigToDefaultCategories(yamlData.Categories)
	if err != nil {
		validationErrs = append(validationErrs, err)
	}

	defaultEntries, err := mapYamlConfigToDefaultEntries(yamlData.DefaultEntries)
	if err != nil {
		validationErrs = append(validationErrs, err)
	}

	return &Configuration{
		LogErr:            errors.Join(validationErrs...),
		DefaultCategories: defaultCategories.InsertEntries(defaultEntries),
	}, nil
}

func mapYamlConfigToDefaultEntries(entries map[string]yaml.Node) (types2.EntriesWithCategory, error) {
	var mappingErrs []error
	var defaultEntries types2.EntriesWithCategory

	if len(entries) == 0 {
		logger.Info("Default entries in warp menu config not defined, skipping them.")
		return defaultEntries, nil
	}

	for entryName, entryNode := range entries {
		var eDTO EntryDTO
		if err := entryNode.Decode(&eDTO); err != nil {
			mappingErrs = append(mappingErrs, fmt.Errorf("failed to decode entry %s: %w", entryName, err))
			continue
		}

		if !eDTO.Enabled {
			continue
		}

		if len(eDTO.Category) == 0 {
			mappingErrs = append(mappingErrs, fmt.Errorf("category is empty in entry %s", entryName))
			continue
		}

		entryURL, err := url.Parse(eDTO.Link)
		if err != nil {
			mappingErrs = append(mappingErrs, fmt.Errorf("invalid link in entry %s: %w", entryName, err))
			continue
		}

		defaultEntry := types2.EntryWithCategory{
			Category: eDTO.Category,
			Entry: types2.Entry{
				Identifier:   entryName,
				Localization: make(types2.LocalizationMap),
				Href:         eDTO.Link,
				Target:       types2.TARGET_SELF,
			},
		}

		if entryURL.IsAbs() {
			defaultEntry.Target = types2.TARGET_EXTERNAL
		}

		var localeErrs []error
		defaultEntry.Localization, localeErrs = mapToLocalizationMap(eDTO.DisplayName, entryName, "entry")
		if len(localeErrs) > 0 {
			mappingErrs = append(mappingErrs, localeErrs...)
		}

		defaultEntries = append(defaultEntries, defaultEntry)
	}

	return defaultEntries, errors.Join(mappingErrs...)
}

func mapYamlConfigToDefaultCategories(categories map[string]yaml.Node) (types2.Categories, error) {
	var mappingErrs []error
	var defaultCategories types2.Categories

	if len(categories) == 0 {
		logger.Info("Default Categories in warp menu config not defined, skipping them.")
		return defaultCategories, nil
	}

	for categoryName, categoryNode := range categories {
		var cDTO CategoryDTO
		if err := categoryNode.Decode(&cDTO); err != nil {
			mappingErrs = append(mappingErrs, fmt.Errorf("failed to decode category %s: %w", categoryName, err))
			continue
		}

		defaultCategory := types2.CreateCategoryFromIdentifier(categoryName)

		if cDTO.Order != nil {
			defaultCategory.Order = *cDTO.Order
		}

		var localeErrs []error
		defaultCategory.Localization, localeErrs = mapToLocalizationMap(cDTO.DisplayName, categoryName, "category")
		if len(localeErrs) > 0 {
			mappingErrs = append(mappingErrs, localeErrs...)
		}

		defaultCategories = append(defaultCategories, &defaultCategory)
	}

	return defaultCategories, errors.Join(mappingErrs...)
}

func mapToLocalizationMap(displayName DisplayNameDTO, identifier string, contextName string) (types2.LocalizationMap, []error) {
	var errs []error
	localizationMap := make(types2.LocalizationMap)

	for localeString, translation := range displayName {
		locale := types2.LocaleFromString(localeString)
		if locale == types2.LocaleUnknown {
			errs = append(errs, fmt.Errorf("unknown locale %s in %s %s", localeString, contextName, identifier))
			continue
		}
		localizationMap[locale] = translation
	}

	return localizationMap, errs
}

func getEnvlookup(env, errormessage, logMessage string) (string, error) {
	envValue, found := os.LookupEnv(env)
	if !found {
		return "", fmt.Errorf(errormessage, env)
	}
	logger.Info(fmt.Sprintf(logMessage, env))

	return envValue, nil
}

// ReadWatchNamespace returns the namespace the controller should watch, read from
// the WATCH_NAMESPACE environment variable.
func ReadWatchNamespace() (string, error) {
	return getEnvlookup(namespaceEnvVar,
		"failed to read namespace to watch from environment variable [%s], please set the variable and try again",
		"found target namespace: [%s]")
}

// ReadWarpPath returns the filesystem path for warp menu output, read from the
// WARP_PATH environment variable.
func ReadWarpPath() (string, error) {
	return getEnvlookup(warpPathEnvVar,
		"failed to read warp path to watch from environment variable [%s], please set the variable and try again",
		"found target warp path: [%s]")
}

// ReadComponentName returns the name of the controller deployment, read from the
// COMPONENT_NAME environment variable.
func ReadComponentName() (string, error) {
	return getEnvlookup(componentNameEnvVar,
		"failed to read component name from environment variable [%s], please set the variable and try again",
		"found target component name: [%s]")
}

func WarpConfigMapPredicate() predicate.Predicate {
	return predicate.And(
		isWarpConfigMapPredicate(),
		warpConfigMapHasChangedPredicate(),
	)
}

func isWarpConfigMapPredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetName() == WarpConfigMap
	})
}

func warpConfigMapHasChangedPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldCM, okOld := e.ObjectOld.(*corev1.ConfigMap)
			newCM, okNew := e.ObjectNew.(*corev1.ConfigMap)

			if !okOld || !okNew {
				return false
			}

			return !reflect.DeepEqual(oldCM.Data, newCM.Data)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return true
		},
	}
}
