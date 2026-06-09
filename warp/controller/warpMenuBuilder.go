package controller

import (
	"sort"

	warpmenu "github.com/cloudogu/k8s-warp-menu-entry-lib/api/v1"
	"github.com/cloudogu/warp-assets/config"
	"github.com/cloudogu/warp-assets/controller/types"
)

type WarpMenuBuilder struct {
	order config.Order
}

func (b WarpMenuBuilder) buildCategories(entries *warpmenu.WarpMenuEntryList) types.Categories {
	categoryMap := map[string]*types.Category{}

	for _, entry := range entries.Items {
		if entry.Spec.Disabled {
			continue
		}
		categoryName := entry.Spec.Category
		cat, exists := categoryMap[categoryName]
		if !exists {
			cat = &types.Category{
				Title:   categoryName,
				Entries: types.Entries{},
				Order:   b.order[categoryName],
			}
			categoryMap[categoryName] = cat
		}
		cat.Entries = append(cat.Entries, b.buildEntry(entry))
	}
	result := make(types.Categories, 0, len(categoryMap))
	for _, cat := range categoryMap {
		sort.Sort(cat.Entries)
		result = append(result, cat)
	}
	sort.Sort(result)
	return result
}

func (b WarpMenuBuilder) buildEntry(entry warpmenu.WarpMenuEntry) types.Entry {
	return types.Entry{
		DisplayName: entry.Spec.DisplayName.DE,
		Href:        entry.Spec.Path,
		Target:      types.TARGET_SELF,
		Localization: map[string]string{
			"de": entry.Spec.DisplayName.DE,
			"en": entry.Spec.DisplayName.EN,
		},
	}
}
