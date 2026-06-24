package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocale_String(t *testing.T) {
	tests := []struct {
		locale   Locale
		expected string
	}{
		{LocaleDe, "de"},
		{LocaleEn, "en"},
		{LocaleUnknown, "unknown"},
		{Locale(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.locale.String())
		})
	}
}

func TestLocaleFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected Locale
	}{
		{"de", LocaleDe},
		{"DE", LocaleDe},
		{"De", LocaleDe},
		{"en", LocaleEn},
		{"EN", LocaleEn},
		{"En", LocaleEn},
		{"fr", LocaleUnknown},
		{"", LocaleUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, LocaleFromString(tt.input))
		})
	}
}

func TestTranslationMap_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    TranslationMap
		expected map[string]string
	}{
		{
			name:     "both locales",
			input:    TranslationMap{LocaleDe: "Hallo", LocaleEn: "Hello"},
			expected: map[string]string{"de": "Hallo", "en": "Hello"},
		},
		{
			name:     "only german",
			input:    TranslationMap{LocaleDe: "Hallo"},
			expected: map[string]string{"de": "Hallo"},
		},
		{
			name:     "only english",
			input:    TranslationMap{LocaleEn: "Hello"},
			expected: map[string]string{"en": "Hello"},
		},
		{
			name:     "empty map",
			input:    TranslationMap{},
			expected: map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			require.NoError(t, err)

			var got map[string]string
			require.NoError(t, json.Unmarshal(data, &got))
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestTranslationMapFromIdentifier(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"myapp"},
		{""},
		{"some-identifier_123"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := TranslationMapFromIdentifier(tt.input)
			assert.Equal(t, tt.input, result[LocaleDe])
			assert.Equal(t, tt.input, result[LocaleEn])
		})
	}
}
