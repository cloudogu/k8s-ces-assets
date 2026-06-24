package types

import (
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
			name:     "higher order sorts before lower order",
			a:        &Category{Order: 1},
			b:        &Category{Order: 100},
			expected: false,
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
	assert.NotNil(t, cat.DisplayName, "DisplayName must be initialised")
	assert.Empty(t, cat.DisplayName)
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
	supportDisplayName := TranslationMap{LocaleDe: "Support", LocaleEn: "Support"}
	appsDisplayName := TranslationMap{LocaleDe: "Apps", LocaleEn: "Apps"}

	tests := []struct {
		name       string
		categories Categories
		entries    EntriesWithCategory
		check      func(t *testing.T, result Categories)
	}{
		{
			name: "entry goes into existing matching category",
			categories: Categories{
				{Identifier: "support", DisplayName: supportDisplayName, Order: 400},
			},
			entries: EntriesWithCategory{
				{Category: "support", Entry: Entry{Identifier: "docs", Href: "/docs"}},
			},
			check: func(t *testing.T, result Categories) {
				assert.Len(t, result, 1)
				assert.Equal(t, "support", result[0].Identifier)
				assert.Equal(t, supportDisplayName, result[0].DisplayName, "DisplayName must be preserved from existing category")
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
				{Identifier: "apps", DisplayName: appsDisplayName, Order: 100},
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
			name: "result categories are sorted by order descending",
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
				assert.Equal(t, "high", result[0].Identifier, "higher order must come first")
				assert.Equal(t, "low", result[1].Identifier)
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
