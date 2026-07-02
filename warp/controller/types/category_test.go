package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCategories_Len(t *testing.T) {
	categories := Categories{&Category{}, &Category{}}
	assert.Equal(t, 2, categories.Len())
}

func TestCategories_Less(t *testing.T) {
	tests := []struct {
		name     string
		a, b     *Category
		expected bool
	}{
		{
			name:     "lower order sorts before higher order",
			a:        &Category{Order: 1},
			b:        &Category{Order: 100},
			expected: true,
		},
		{
			name:     "equal order falls back to identifier ascending",
			a:        &Category{Order: 100, Identifier: "A"},
			b:        &Category{Order: 100, Identifier: "B"},
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			categories := Categories{tt.a, tt.b}
			assert.Equal(t, tt.expected, categories.Less(0, 1))
		})
	}
}

func TestCategories_Swap(t *testing.T) {
	a := &Category{Order: 1}
	b := &Category{Order: 100}
	categories := Categories{a, b}

	categories.Swap(0, 1)

	assert.Equal(t, b, categories[0])
	assert.Equal(t, a, categories[1])
}

func TestCategories_InsertCategories(t *testing.T) {
	a := &Category{Order: 1, Identifier: "a"}
	b := &Category{Order: 100, Identifier: "b"}
	categories := Categories{a, b}
	aa := &Category{Order: 1, Identifier: "aa"}
	bb := &Category{Order: 100, Identifier: "bb"}

	categories.InsertCategories(Categories{aa, bb})

	assert.Equal(t, 4, len(categories))
	assert.Equal(t, a, categories[0])
	assert.Equal(t, b, categories[1])
	assert.Equal(t, aa, categories[2])
	assert.Equal(t, bb, categories[3])
}

func TestCreateCategoryFromIdentifier(t *testing.T) {
	cat := CreateCategoryFromIdentifier("myapp")

	assert.Equal(t, "myapp", cat.Identifier)
	assert.Equal(t, defaultCategoryOrder, cat.Order)
	assert.NotNil(t, cat.Localization, "Localization must be initialised")
	assert.Empty(t, cat.Localization)
	assert.NotNil(t, cat.Entries, "Entries must be initialised")
	assert.Empty(t, cat.Entries)
}

func TestCategories_InsertCategory(t *testing.T) {
	t.Run("new identifier does not exist — appends category", func(t *testing.T) {
		a := &Category{Order: 1, Identifier: "a"}
		b := &Category{Order: 100, Identifier: "b"}
		categories := Categories{a, b}
		add := &Category{Order: 50, Identifier: "c"}

		categories.InsertCategory(add)

		assert.Equal(t, 3, len(categories))
		assert.Equal(t, add, categories[2])
	})

	t.Run("matching identifier — merges entries instead of appending category", func(t *testing.T) {
		existingEntry := Entry{Identifier: "existing"}
		a := &Category{Order: 1, Identifier: "a", Entries: Entries{existingEntry}}
		b := &Category{Order: 100, Identifier: "b"}
		categories := Categories{a, b}
		newEntry := Entry{Identifier: "new"}
		add := &Category{Order: 50, Identifier: "a", Entries: Entries{newEntry}}

		categories.InsertCategory(add)

		assert.Equal(t, 2, len(categories))
		assert.Equal(t, existingEntry, categories[0].Entries[0])
		assert.Equal(t, newEntry, categories[0].Entries[1])
	})
}

func TestCategories_InsertEntries(t *testing.T) {
	supportDisplayName := LocalizationMap{LocaleDe: "Support", LocaleEn: "Support"}
	appsDisplayName := LocalizationMap{LocaleDe: "Apps", LocaleEn: "Apps"}

	tests := []struct {
		name       string
		categories Categories
		entries    EntriesWithCategory
		check      func(t *testing.T, result Categories)
	}{
		{
			name: "entry goes into existing matching category",
			categories: Categories{
				{Identifier: "support", Localization: supportDisplayName, Order: 400},
			},
			entries: EntriesWithCategory{
				{Category: "support", Entry: Entry{Identifier: "docs", Href: "/docs"}},
			},
			check: func(t *testing.T, result Categories) {
				assert.Len(t, result, 1)
				assert.Equal(t, "support", result[0].Identifier)
				assert.Equal(t, supportDisplayName, result[0].Localization, "DisplayName must be preserved from existing category")
				assert.Equal(t, 400, result[0].Order, "Order must be preserved from existing category")
				assert.Len(t, result[0].Entries, 1)
				assert.Equal(t, "docs", result[0].Entries[0].Identifier)
			},
		},
		{
			name:       "unknown category identifier creates new category with default order",
			categories: Categories{},
			entries: EntriesWithCategory{
				{Category: "unknown", Entry: Entry{Identifier: "tool"}},
			},
			check: func(t *testing.T, result Categories) {
				assert.Len(t, result, 1)
				assert.Equal(t, "unknown", result[0].Identifier)
				assert.Equal(t, defaultCategoryOrder, result[0].Order)
				assert.Len(t, result[0].Entries, 1)
			},
		},
		{
			name: "entries within a category are sorted by identifier",
			categories: Categories{
				{Identifier: "apps", Localization: appsDisplayName, Order: 100},
			},
			entries: EntriesWithCategory{
				{Category: "apps", Entry: Entry{Identifier: "z-tool"}},
				{Category: "apps", Entry: Entry{Identifier: "a-tool"}},
			},
			check: func(t *testing.T, result Categories) {
				require.Len(t, result[0].Entries, 2)
				assert.Equal(t, "a-tool", result[0].Entries[0].Identifier)
				assert.Equal(t, "z-tool", result[0].Entries[1].Identifier)
			},
		},
		{
			name: "result categories are sorted by order ascending",
			categories: Categories{
				{Identifier: "low", Order: 10},
				{Identifier: "high", Order: 500},
			},
			entries: EntriesWithCategory{
				{Category: "low", Entry: Entry{Identifier: "e1"}},
				{Category: "high", Entry: Entry{Identifier: "e2"}},
			},
			check: func(t *testing.T, result Categories) {
				require.Len(t, result, 2)
				assert.Equal(t, "low", result[0].Identifier, "lower order must come first")
				assert.Equal(t, "high", result[1].Identifier)
			},
		},
		{
			name: "empty entries returns original categories unchanged",
			categories: Categories{
				{Identifier: "support", Order: 400},
			},
			entries: EntriesWithCategory{},
			check: func(t *testing.T, result Categories) {
				assert.Len(t, result, 1)
				assert.Equal(t, "support", result[0].Identifier)
				assert.Empty(t, result[0].Entries)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.categories.InsertEntries(tt.entries)
			tt.check(t, result)
		})
	}
}

func TestCategory_MarshalJSON(t *testing.T) {
	type jsonEntry struct {
		Title        string            `json:"Title"`
		DisplayName  string            `json:"DisplayName"`
		Href         string            `json:"Href"`
		Target       string            `json:"Target"`
		Localization map[string]string `json:"Localization"`
	}
	type jsonCategory struct {
		Title        string            `json:"Title"`
		Order        int               `json:"Order"`
		Localization map[string]string `json:"Localization"`
		Entries      []jsonEntry       `json:"Entries"`
	}

	tests := []struct {
		name     string
		input    Category
		expected jsonCategory
	}{
		{
			name: "full category with entries",
			input: Category{
				Identifier:   "apps",
				Localization: LocalizationMap{LocaleDe: "Anwendungen", LocaleEn: "Applications"},
				Order:        500,
				Entries: Entries{
					{Identifier: "my-app", Localization: LocalizationMap{LocaleDe: "Meine App", LocaleEn: "My App"}, Href: "/apps/my-app", Target: TARGET_SELF},
				},
			},
			expected: jsonCategory{
				Title:        "apps",
				Localization: map[string]string{"de": "Anwendungen", "en": "Applications"},
				Entries:      []jsonEntry{{Title: "my-app", DisplayName: "Meine App", Href: "/apps/my-app", Target: "self", Localization: map[string]string{"de": "Meine App", "en": "My App"}}},
			},
		},
		{
			name: "category with empty DisplayName and no entries",
			input: Category{
				Identifier:   "empty",
				Localization: LocalizationMap{},
				Order:        defaultCategoryOrder,
				Entries:      Entries{},
			},
			expected: jsonCategory{
				Title:        "empty",
				Localization: map[string]string{},
				Entries:      []jsonEntry{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			require.NoError(t, err)

			var got jsonCategory
			require.NoError(t, json.Unmarshal(data, &got))
			assert.Equal(t, tt.expected, got)
		})
	}
}
