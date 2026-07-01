package types

import (
	"encoding/json"
	"math"
	"sort"
)

const (
	// defaultCategoryOrder is set to math.MaxInt so unconfigured categories appear at the end of the menu.
	defaultCategoryOrder = math.MaxInt
)

// Category groups related entries under a named section in the warp menu.
type Category struct {
	// Identifier is the locale-independent key used for deduplication and sorting.
	Identifier string
	// DisplayName holds the translated category name for each supported locale.
	Localization LocalizationMap
	// Order controls the display position; lower values appear first (higher up in the menu).
	Order int
	// Entries is the ordered list of links belonging to this category.
	Entries Entries
}

// CreateCategoryFromIdentifier builds a Category with a default Order (9999)
// and an initialised but empty DisplayName and Entries. Use this when a
// category is implied by an entry's tag but has no explicit configuration.
func CreateCategoryFromIdentifier(identifier string) Category {
	return Category{
		Identifier:   identifier,
		Localization: LocalizationMapFromIdentifier(identifier),
		Order:        defaultCategoryOrder,
		Entries:      make(Entries, 0),
	}
}

// Categories is an ordered collection of Category pointers.
type Categories []*Category

// Len implements sort.Interface.
func (c Categories) Len() int {
	return len(c)
}

// Less implements sort.Interface. Categories with a lower Order appear first;
// ties are broken by Identifier ascending.
func (c Categories) Less(i, j int) bool {
	if c[i].Order == c[j].Order {
		return c[i].Identifier < c[j].Identifier
	}
	return c[i].Order < c[j].Order
}

// Swap implements sort.Interface.
func (c Categories) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

// InsertCategories adds new categories to the slice.
func (c *Categories) InsertCategories(newCategories Categories) {
	for _, newCategory := range newCategories {
		c.InsertCategory(newCategory)
	}
}

// InsertCategory adds a new category to the slice. If the identifier are same the entries will be merged.
func (c *Categories) InsertCategory(newCategory *Category) {
	for _, category := range *c {
		if category.Identifier == newCategory.Identifier {
			category.Entries = append(category.Entries, newCategory.Entries...)
			return
		}
	}
	*c = append(*c, newCategory)
}

// InsertEntries merges newEntries into the receiver Categories and returns a
// new sorted Categories. Each entry carries a category Identifier; if a
// matching Category exists in c its entries are extended, otherwise a new
// Category is created via CreateCategoryFromIdentifier. Entries within each
// Category are sorted by Identifier; Categories are sorted by Order descending.
func (c Categories) InsertEntries(newEntries EntriesWithCategory) Categories {
	categoryMap := make(map[string]*Category, c.Len())
	for _, category := range c {
		categoryMap[category.Identifier] = category
	}

	for _, entry := range newEntries {
		categoryName := entry.Category
		cat, exists := categoryMap[categoryName]
		if !exists {
			cat = new(CreateCategoryFromIdentifier(categoryName))
			categoryMap[categoryName] = cat
		}
		cat.Entries = append(cat.Entries, entry.Entry)
	}

	result := make(Categories, 0, len(categoryMap))
	for _, cat := range categoryMap {
		sort.Sort(cat.Entries)
		result = append(result, cat)
	}

	sort.Sort(result)

	return result
}

// MarshalJSON serialises Category using its API-facing JSON shape: Title maps
// to Identifier and Translations to DisplayName, avoiding exposure of internal
// field names in the JSON output.
func (c Category) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Title        string          `json:"Title"`
		Localization LocalizationMap `json:"Localization"`
		Entries      Entries         `json:"Entries"`
	}{
		Title:        c.Identifier,
		Localization: c.Localization,
		Entries:      c.Entries,
	})
}
