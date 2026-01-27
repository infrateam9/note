package main

import (
	"regexp"
	"testing"
)

// TestValidateNoteID tests note ID validation
func TestValidateNoteID(t *testing.T) {
	tests := []struct {
		id    string
		valid bool
	}{
		{"abc123", true},
		{"ABC", true},
		{"123", true},
		{"a", true},
		{"", false},
		{"abc-def", false},
		{"abc_def", false},
		{"abc@def", false},
		{"abc def", false},
		{"abc.def", false},
		{"../etc", false},
	}

	for _, test := range tests {
		result := ValidateNoteID(test.id)
		if result != test.valid {
			t.Errorf("ValidateNoteID(%s) = %v, expected %v", test.id, result, test.valid)
		}
	}
}

// TestGenerateNoteID tests note ID generation
func TestGenerateNoteID(t *testing.T) {
	tests := 100

	for i := 0; i < tests; i++ {
		id := GenerateNoteID()

		// Check length
		if len(id) != 5 {
			t.Errorf("Generated ID has length %d, expected 5", len(id))
		}

		// Check alphanumeric
		if !regexp.MustCompile("^[a-zA-Z0-9]+$").MatchString(id) {
			t.Errorf("Generated ID %s is not alphanumeric", id)
		}

		// Check it's valid
		if !ValidateNoteID(id) {
			t.Errorf("Generated ID %s is not valid", id)
		}
	}

	// Check uniqueness (probabilistic)
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := GenerateNoteID()
		if ids[id] {
			t.Logf("Warning: Generated duplicate ID %s (may be rare)", id)
		}
		ids[id] = true
	}
}

// TestEscapeHTML tests HTML escaping
func TestEscapeHTML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<script>alert('xss')</script>", "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;"},
		{"<div>test</div>", "&lt;div&gt;test&lt;/div&gt;"},
		{"normal text", "normal text"},
		{"test & test", "test &amp; test"},
		{"\"quoted\"", "&quot;quoted&quot;"},
		{"'quoted'", "&#39;quoted&#39;"},
		{"", ""},
	}

	for _, test := range tests {
		result := EscapeHTML(test.input)
		if result != test.expected {
			t.Errorf("EscapeHTML(%s) = %s, expected %s", test.input, result, test.expected)
		}
	}
}
