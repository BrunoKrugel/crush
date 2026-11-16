package diffview

import (
	"testing"
)

func TestPad(t *testing.T) {
	tests := []struct {
		input    any
		expected string
		width    int
	}{
		{input: 7, width: 2, expected: " 7"},
		{input: 7, width: 3, expected: "  7"},
		{input: "a", width: 2, expected: " a"},
		{input: "a", width: 3, expected: "  a"},
		{input: "…", width: 2, expected: " …"},
		{input: "…", width: 3, expected: "  …"},
	}

	for _, tt := range tests {
		result := pad(tt.input, tt.width)
		if result != tt.expected {
			t.Errorf("expected %q, got %q", tt.expected, result)
		}
	}
}
