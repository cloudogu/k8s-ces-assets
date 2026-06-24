package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
