package controller

import (
	"testing"

	warpmenu "github.com/cloudogu/k8s-warp-menu-entry-lib/api/v1"
	"github.com/cloudogu/warp-assets/config"
	"github.com/cloudogu/warp-assets/controller/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildWarpMenu(t *testing.T) {

	t.Run("should build empty menu with empty entry list", func(t *testing.T) {
		builder := WarpMenuBuilder{config.Order{}}

		categories, err := builder.buildCategories(&warpmenu.WarpMenuEntryList{})
		if err != nil {
			require.NoError(t, err)
		}
		assert.Equal(t, 0, len(categories))
	})

	t.Run("should build categories for simple entry list", func(t *testing.T) {
		builder := WarpMenuBuilder{config.Order{}}

		categories, err := builder.buildCategories(&warpmenu.WarpMenuEntryList{
			Items: []warpmenu.WarpMenuEntry{
				buildWarpMenuEntry("Category", "/jenkins", "Jenkins DE", "Jenkins EN", false),
			},
		})
		if err != nil {
			require.NoError(t, err)
		}
		checkCategories(t, categories, 1, "Category")
		assert.Equal(t, 1, len(categories[0].Entries))
		checkEntry(t, categories[0].Entries[0], "Jenkins DE", "/jenkins")
	})

	t.Run("should build categories for a complex entry list", func(t *testing.T) {
		builder := WarpMenuBuilder{config.Order{}}

		categories, err := builder.buildCategories(&warpmenu.WarpMenuEntryList{
			Items: []warpmenu.WarpMenuEntry{
				buildWarpMenuEntry("Category A", "/alpha", "Jenkins A", "Jenkins A EN", false),
				buildWarpMenuEntry("Category B", "/beta", "Jenkins B", "Jenkins B EN", false),
				buildWarpMenuEntry("Category A", "/aleph", "Jenkins A2", "Jenkins A2 EN", false),
			},
		})
		if err != nil {
			require.NoError(t, err)
		}
		checkCategories(t, categories, 2, "Category A", "Category B")
		assert.Equal(t, 2, len(categories[0].Entries))
		checkEntry(t, categories[0].Entries[0], "Jenkins A", "/alpha")
		checkEntry(t, categories[0].Entries[1], "Jenkins A2", "/aleph")
		assert.Equal(t, 1, len(categories[1].Entries))
		checkEntry(t, categories[1].Entries[0], "Jenkins B", "/beta")
	})

	t.Run("should omit disabled entries", func(t *testing.T) {
		builder := WarpMenuBuilder{config.Order{}}

		categories, err := builder.buildCategories(&warpmenu.WarpMenuEntryList{
			Items: []warpmenu.WarpMenuEntry{
				buildWarpMenuEntry("Category A", "/alpha", "Jenkins A", "Jenkins A EN", true),
				buildWarpMenuEntry("Category B", "/beta", "Jenkins B", "Jenkins B EN", false),
				buildWarpMenuEntry("Category A", "/aleph", "Jenkins A2", "Jenkins A2 EN", false),
			},
		})
		if err != nil {
			require.NoError(t, err)
		}
		checkCategories(t, categories, 2, "Category A", "Category B")
		assert.Equal(t, 1, len(categories[0].Entries))
		checkEntry(t, categories[0].Entries[0], "Jenkins A2", "/aleph")
		assert.Equal(t, 1, len(categories[1].Entries))
		checkEntry(t, categories[1].Entries[0], "Jenkins B", "/beta")
	})
}

func checkEntry(t *testing.T, entry types.Entry, expectedDisplayName, expectedPath string) {
	assert.Equal(t, expectedDisplayName, entry.DisplayName)
	assert.Equal(t, expectedPath, entry.Href)
	assert.Equal(t, types.TARGET_SELF, entry.Target)
}

func checkCategories(t *testing.T, categories types.Categories, expectedLength int, expectedNames ...string) {
	assert.Equal(t, expectedLength, len(categories))
	for i, category := range categories {
		assert.Equal(t, expectedNames[i], category.Title)
	}
}

func buildWarpMenuEntry(category, path, displayNameDe, displayNameEn string, disabled bool) warpmenu.WarpMenuEntry {
	return warpmenu.WarpMenuEntry{
		Spec: warpmenu.WarpMenuEntrySpec{
			Category: category,
			Path:     path,
			DisplayName: warpmenu.DisplayName{
				DE: displayNameDe,
				EN: displayNameEn,
			},
			Disabled: disabled,
		},
	}
}
