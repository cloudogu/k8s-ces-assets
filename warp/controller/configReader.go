package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	libconfig "github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/warp-assets/config"
	types2 "github.com/cloudogu/warp-assets/controller/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/pkg/errors"
)

// ConfigReader reads the configuration for the warp menu from the global configuration
type ConfigReader struct {
	configuration       *config.Configuration
	globalConfigRepo    GlobalConfigRepository
	doguVersionRegistry DoguVersionRegistry
	localDoguRepo       LocalDoguRepo
	doguConverter       DoguConverter
	externalConverter   ExternalConverter
}

const globalBlockWarpSupportCategoryConfigurationKey = "block_warpmenu_support_category"
const globalDisabledWarpSupportEntriesConfigurationKey = "disabled_warpmenu_support_entries"
const globalAllowedWarpSupportEntriesConfigurationKey = "allowed_warpmenu_support_entries"

func NewConfigReader(
	warpMenuConfiguration *config.Configuration,
	globalConfigRepo GlobalConfigRepository,
	doguVersionRegistry DoguVersionRegistry,
	localDoguRepo LocalDoguRepo,
) *ConfigReader {
	return &ConfigReader{
		configuration:       warpMenuConfiguration,
		globalConfigRepo:    globalConfigRepo,
		doguVersionRegistry: doguVersionRegistry,
		localDoguRepo:       localDoguRepo,
		doguConverter:       &types2.DoguConverter{},
		externalConverter:   &types2.ExternalConverter{},
	}
}

// Read reads sources specified in a configuration and build warp menu categories for them.
func (reader *ConfigReader) Read(ctx context.Context, configuration *config.Configuration) (types2.Categories, error) {
	var data types2.Categories

	for _, source := range configuration.Sources {
		// Disabled support entries refresh every time
		if source.Type == "support_entry_config" {
			continue
		}

		categories, err := reader.readSource(ctx, source)
		if err != nil {
			ctrl.Log.Info(fmt.Sprintf("Error during Read: %s", err.Error()))
		}
		data.InsertCategories(categories)
	}

	ctrl.Log.Info("Read SupportEntries")

	readKeyErrorFmt := "Warning, could not read Key: %v. Err: %v"

	isSupportCategoryBlocked, err := reader.readBool(ctx, globalBlockWarpSupportCategoryConfigurationKey)
	if err != nil {
		ctrl.Log.Info(fmt.Sprintf(readKeyErrorFmt, globalBlockWarpSupportCategoryConfigurationKey, err))
	}

	disabledSupportEntries, err := reader.readStrings(ctx, globalDisabledWarpSupportEntriesConfigurationKey)
	if err != nil {
		ctrl.Log.Info(fmt.Sprintf(readKeyErrorFmt, globalDisabledWarpSupportEntriesConfigurationKey, err))
	}

	allowedSupportEntries, err := reader.readStrings(ctx, globalAllowedWarpSupportEntriesConfigurationKey)
	if err != nil {
		ctrl.Log.Info(fmt.Sprintf(readKeyErrorFmt, globalAllowedWarpSupportEntriesConfigurationKey, err))
	}

	supportCategory := reader.readSupport(configuration.Support, isSupportCategoryBlocked, disabledSupportEntries, allowedSupportEntries)
	data.InsertCategories(supportCategory)
	return data, nil
}

func (reader *ConfigReader) readSource(ctx context.Context, source config.Source) (types2.Categories, error) {
	switch source.Type {
	case "dogus":
		return reader.dogusReader(ctx, source)
	case "externals":
		return reader.externalsReader(ctx, source)
	}
	return nil, errors.Errorf("wrong source type: %v", source.Type)
}

func (reader *ConfigReader) externalsReader(ctx context.Context, source config.Source) (types2.Categories, error) {
	ctrl.Log.Info(fmt.Sprintf("Read externals from %s for warp menu in global config", source.Path))
	children, err := reader.readGlobalConfigDir(ctx, removeLegacyGlobalConfigPrefix(source.Path))
	if err != nil {
		return nil, fmt.Errorf("failed to read root entry %s from config: %w", source.Path, err)
	}
	var externals []types2.EntryWithCategory
	for _, value := range children {
		external, unmarshalErr := reader.externalConverter.ReadAndUnmarshalExternal(value)
		if unmarshalErr != nil {
			ctrl.Log.Error(unmarshalErr, fmt.Sprintf("failed to read and unmarshal external link key %q", value))
			continue
		}
		externals = append(externals, external)
	}
	return reader.createCategories(externals), nil
}

func (reader *ConfigReader) readGlobalConfigDir(ctx context.Context, key string) (map[string]string, error) {
	globalConfig, err := reader.getGlobalConfig(ctx)
	if err != nil {
		return nil, err
	}

	entries := globalConfig.GetAll()
	children := make(map[string]string, len(entries))
	for entryKey, entryValue := range entries {
		if strings.HasPrefix(entryKey.String(), key) && entryKey.String() != key {
			children[entryKey.String()] = entryValue.String()
		}
	}

	return children, nil
}

// dogusReader reads from dogu repository and converts the keys and values to a warp menu
// conform structure
func (reader *ConfigReader) dogusReader(ctx context.Context, source config.Source) (types2.Categories, error) {
	ctrl.Log.Info(fmt.Sprintf("Read dogus from %s for warp menu", source.Path))
	allCurrentDoguVersions, err := reader.doguVersionRegistry.GetCurrentOfAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all current dogu versions: %w", err)
	}

	if len(allCurrentDoguVersions) == 0 {
		return []*types2.Category{}, nil
	}

	allCurrentDogus, err := reader.localDoguRepo.GetAll(ctx, allCurrentDoguVersions)
	if err != nil {
		return nil, fmt.Errorf("failed to get all dogu specs with current versions: %w", err)
	}

	var doguCategories []types2.EntryWithCategory
	for _, currentDogu := range allCurrentDogus {
		doguCategory, err := reader.doguConverter.CreateEntryWithCategoryFromDogu(currentDogu, source.Tag)
		if err == nil && doguCategory.Entry.Title != "" {
			ctrl.Log.Info(fmt.Sprintf("Add dogu %s with category %s", currentDogu.GetSimpleName(), doguCategory.Category))
			doguCategories = append(doguCategories, doguCategory)
		}
	}

	return reader.createCategories(doguCategories), nil
}

func (reader *ConfigReader) readStrings(ctx context.Context, registryKey string) ([]string, error) {
	globalConfig, err := reader.getGlobalConfig(ctx)
	if err != nil {
		return nil, err
	}

	entry, exists := globalConfig.Get(libconfig.Key(registryKey))
	if !exists || !ContainsChars(entry.String()) {
		return []string{}, nil
	}

	var stringSlice []string
	err = json.Unmarshal([]byte(entry.String()), &stringSlice)
	if err != nil {
		return []string{}, fmt.Errorf("failed to unmarshal global config key to string slice: %w", err)
	}

	return stringSlice, nil
}

func removeLegacyGlobalConfigPrefix(key string) string {
	if strings.HasPrefix(key, "config/_global") || strings.HasPrefix(key, "/config/_global") {
		_, after, _ := strings.Cut(key, "config/_global/")
		return after
	}

	return key
}

func (reader *ConfigReader) getGlobalConfig(ctx context.Context) (libconfig.GlobalConfig, error) {
	globalConfig, err := reader.globalConfigRepo.Get(ctx)
	if err != nil {
		return libconfig.GlobalConfig{}, fmt.Errorf("failed to get global config: %w", err)
	}

	return globalConfig, nil
}

func (reader *ConfigReader) readBool(ctx context.Context, registryKey string) (bool, error) {
	globalConfig, err := reader.getGlobalConfig(ctx)
	if err != nil {
		return false, err
	}

	entry, exists := globalConfig.Get(libconfig.Key(registryKey))
	if !exists || !ContainsChars(entry.String()) {
		return false, nil
	}

	boolValue, err := strconv.ParseBool(entry.String())
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal value %q to bool: %w", entry, err)
	}

	return boolValue, nil
}

func (reader *ConfigReader) readSupport(supportSources []config.SupportSource, blocked bool, disabledEntries []string, allowedEntries []string) types2.Categories {
	var supportEntries []types2.EntryWithCategory

	for _, supportSource := range supportSources {
		if (blocked && StringInSlice(supportSource.Identifier, allowedEntries)) || (!blocked && !StringInSlice(supportSource.Identifier, disabledEntries)) {
			// support category is blocked, but this entry is explicitly allowed OR support category is NOT blocked and this entry is NOT explicitly disabled

			entry := types2.Entry{Title: supportSource.Identifier, Href: supportSource.Href, Target: types2.TARGET_SELF}
			if supportSource.External {
				entry.Target = types2.TARGET_EXTERNAL
			}

			supportEntries = append(supportEntries, types2.EntryWithCategory{Entry: entry, Category: "Support"})
		}
	}

	return reader.createCategories(supportEntries)
}

func (reader *ConfigReader) createCategories(entries []types2.EntryWithCategory) types2.Categories {
	categories := map[string]*types2.Category{}

	for _, entry := range entries {
		categoryName := entry.Category
		category := categories[categoryName]
		if category == nil {
			category = &types2.Category{
				Title:   categoryName,
				Entries: types2.Entries{},
				Order:   reader.configuration.Order[categoryName],
			}
			categories[categoryName] = category
		}
		category.Entries = append(category.Entries, entry.Entry)
	}

	result := types2.Categories{}
	for _, cat := range categories {
		sort.Sort(cat.Entries)
		result = append(result, cat)
	}
	sort.Sort(result)
	return result
}

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func ContainsChars(s string) bool {
	return len(strings.TrimSpace(s)) != 0
}
