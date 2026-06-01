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

func (b WarpMenuBuilder) buildCategories(entries *warpmenu.WarpMenuEntryList) (types.Categories, error) {
	var list []types.EntryWithCategory
	for _, entry := range entries.Items {
		if !entry.Spec.Disabled {
			list = append(list, b.buildEntryWithCategory(entry))
		}
	}
	return b.convertToCategories(list), nil
}

func (b WarpMenuBuilder) buildEntryWithCategory(entry warpmenu.WarpMenuEntry) types.EntryWithCategory {
	return types.EntryWithCategory{
		Category: entry.Spec.Category,
		Entry: types.Entry{
			DisplayName: entry.Spec.DisplayName.DE,
			Href:        entry.Spec.Path,
			Target:      types.TARGET_SELF,
			Localization: map[string]string{
				"de": entry.Spec.DisplayName.DE,
				"en": entry.Spec.DisplayName.EN,
			}},
	}
}

func (b WarpMenuBuilder) convertToCategories(entries []types.EntryWithCategory) types.Categories {
	categories := map[string]*types.Category{}

	for _, entry := range entries {
		categoryName := entry.Category
		category := categories[categoryName]
		if category == nil {
			category = &types.Category{
				Title:   categoryName,
				Entries: types.Entries{},
				Order:   b.order[categoryName],
			}
			categories[categoryName] = category
		}
		category.Entries = append(category.Entries, entry.Entry)
	}

	result := types.Categories{}
	for _, cat := range categories {
		sort.Sort(cat.Entries)
		result = append(result, cat)
	}
	sort.Sort(result)
	return result

}
