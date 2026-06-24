package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/json"
)

func TestTarget_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		target   Target
		expected string
		wantErr  bool
	}{
		{name: "self", target: TARGET_SELF, expected: `{"Target":"self"}`},
		{name: "external", target: TARGET_EXTERNAL, expected: `{"Target":"external"}`},
		{name: "unknown value returns error", target: Target(12), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(targetStruct{tt.target})
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, string(data))
		})
	}
}

type targetStruct struct {
	Target Target
}

func TestEntries_Len(t *testing.T) {
	entries := Entries{Entry{}, Entry{}}
	assert.Equal(t, 2, entries.Len())
}

func TestEntries_Less(t *testing.T) {
	tests := []struct {
		name     string
		a, b     Entry
		expected bool
	}{
		{
			name:     "a comes before b alphabetically",
			a:        Entry{Identifier: "A"},
			b:        Entry{Identifier: "B"},
			expected: true,
		},
		{
			name:     "b comes before a alphabetically",
			a:        Entry{Identifier: "B"},
			b:        Entry{Identifier: "A"},
			expected: false,
		},
		{
			name:     "equal identifiers",
			a:        Entry{Identifier: "A"},
			b:        Entry{Identifier: "A"},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := Entries{tt.a, tt.b}
			assert.Equal(t, tt.expected, entries.Less(0, 1))
		})
	}
}

func TestEntries_Swap(t *testing.T) {
	entry1 := Entry{Identifier: "1"}
	entry2 := Entry{Identifier: "2"}
	entries := Entries{entry1, entry2}

	entries.Swap(0, 1)

	assert.Equal(t, entry2, entries[0])
	assert.Equal(t, entry1, entries[1])
}
