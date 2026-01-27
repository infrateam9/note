package main

import (
	"html"
	"math/rand"
	"regexp"
)

// ValidateNoteID checks if a note ID is valid (alphanumeric only)
func ValidateNoteID(noteID string) bool {
	if noteID == "" {
		return false
	}
	matched, _ := regexp.MatchString("^[a-zA-Z0-9]+$", noteID)
	return matched
}

// GenerateNoteID creates a random 5-character alphanumeric note ID
func GenerateNoteID() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 5
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// EscapeHTML escapes HTML special characters
func EscapeHTML(s string) string {
	return html.EscapeString(s)
}
