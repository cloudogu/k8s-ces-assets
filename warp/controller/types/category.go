package types

// Category groups related entries under a named section in the warp menu.
type Category struct {
	// Identifier is the locale-independent key used for deduplication and sorting.
	Identifier  string
	DisplayName TranslationMap
	// Order controls the display position; higher values appear first.
	Order   int
	Entries Entries
}

// Categories is an ordered collection of Category pointers.
type Categories []*Category

// Len implements sort.Interface.
func (c Categories) Len() int {
	return len(c)
}

// Less implements sort.Interface. Categories with a higher Order appear first;
// ties are broken by Identifier ascending.
func (c Categories) Less(i, j int) bool {
	if c[i].Order == c[j].Order {
		return c[i].Identifier < c[j].Identifier
	}
	return c[i].Order > c[j].Order
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
