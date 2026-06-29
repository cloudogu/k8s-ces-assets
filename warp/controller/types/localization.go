package types

import (
	"encoding/json"
	"strings"
)

// Locale identifies a supported display language.
type Locale uint8

const (
	// LocaleUnknown is returned when no matching locale can be determined.
	LocaleUnknown Locale = iota
	// LocaleDe represents German.
	LocaleDe
	// LocaleEn represents English.
	LocaleEn
)

// String returns the locale tag for the locale ("de", "en", or "unknown").
func (l Locale) String() string {
	switch l {
	case LocaleDe:
		return "de"
	case LocaleEn:
		return "en"
	default:
		return "unknown"
	}
}

// LocaleFromString parses a locale tag (case-insensitive) and returns
// the matching Locale, or LocaleUnknown if the tag is not recognized.
func LocaleFromString(localeString string) Locale {
	switch strings.ToLower(localeString) {
	case "de":
		return LocaleDe
	case "en":
		return LocaleEn
	default:
		return LocaleUnknown
	}
}

// LocalizationMap holds one translated string per Locale.
type LocalizationMap map[Locale]string

// MarshalJSON serialises the map with locale keys so the JSON output is
// human-readable (e.g. {"de":"Hallo","en":"Hello"} instead of numeric keys).
func (d LocalizationMap) MarshalJSON() ([]byte, error) {
	resultMap := make(map[string]string)

	for locale, translation := range d {
		resultMap[locale.String()] = translation
	}

	return json.Marshal(resultMap)
}

// LocalizationMapFromIdentifier returns a LocalizationMap whose "de" and "en"
// values are both set to s. Useful when only a locale-independent identifier is
// available and no separate translations exist yet.
func LocalizationMapFromIdentifier(s string) LocalizationMap {
	return map[Locale]string{
		LocaleDe: s,
		LocaleEn: s,
	}
}
