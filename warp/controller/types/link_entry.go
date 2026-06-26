package types

import (
	"encoding/json"
	"fmt"
)

// EntryWithCategory pairs an Entry with the Identifier of the Category it
// belongs to. Used when routing CRD-sourced entries to their categories.
type EntryWithCategory struct {
	Category string
	Entry
}

// EntriesWithCategory is an ordered slice of EntryWithCategory.
type EntriesWithCategory []EntryWithCategory

// MapToEntries strips the Category field and returns the bare Entries.
func (e EntriesWithCategory) MapToEntries() Entries {
	mapped := make(Entries, len(e))

	for i, entry := range e {
		mapped[i] = entry.Entry
	}

	return mapped
}

// Entry represents a single link in the warp menu.
type Entry struct {
	Identifier  string
	DisplayName TranslationMap
	Href        string
	Target      Target
}

// MarshalJSON serialises Entry using its API-facing JSON shape: Title maps to
// Identifier, DisplayName is the German translation (legacy flat field), and
// Translations carries the full locale map.
func (e Entry) MarshalJSON() ([]byte, error) {
	displayName := e.DisplayName[LocaleDe]

	return json.Marshal(struct {
		Title        string         `json:"Title"`
		DisplayName  string         `json:"DisplayName"`
		Href         string         `json:"Href"`
		Target       Target         `json:"Target"`
		Translations TranslationMap `json:"Translations"`
	}{
		Title:        e.Identifier,
		DisplayName:  displayName,
		Href:         e.Href,
		Target:       e.Target,
		Translations: e.DisplayName,
	})
}

// Target defines where a link opens.
type Target uint8

const (
	// TARGET_SELF opens the link within the current system (internal navigation).
	TARGET_SELF Target = iota + 1
	// TARGET_EXTERNAL opens the link outside the system (external browser tab).
	TARGET_EXTERNAL
)

// MarshalJSON serialises Target as a JSON string ("self" or "external").
// Returns an error for unrecognised values so broken data surfaces early.
func (target Target) MarshalJSON() ([]byte, error) {
	switch target {
	case TARGET_SELF:
		return target.asJSONString("self"), nil
	case TARGET_EXTERNAL:
		return target.asJSONString("external"), nil
	default:
		return nil, fmt.Errorf("unknown target type %d", target)
	}
}

func (target Target) asJSONString(value string) []byte {
	return []byte("\"" + value + "\"")
}

// Entries is an ordered collection of warp menu entries.
type Entries []Entry

// Len implements sort.Interface.
func (e Entries) Len() int {
	return len(e)
}

// Less implements sort.Interface. Entries are sorted by Identifier ascending.
func (e Entries) Less(i, j int) bool {
	return e[i].Identifier < e[j].Identifier
}

// Swap implements sort.Interface.
func (e Entries) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}
