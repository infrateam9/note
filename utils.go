package main

import (
	"html"
	"math/rand"
	"net"
	"net/http"
	"regexp"
	"strings"
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
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
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

// ClientIP attempts to determine the real client IP when running behind proxies.
// Preference order:
// - Forwarded: for=...
// - X-Forwarded-For: first IP in list
// - X-Real-IP
// - r.RemoteAddr
func ClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}

	if fwd := r.Header.Get("Forwarded"); fwd != "" {
		// Example: Forwarded: for=203.0.113.60;proto=https;by=203.0.113.43
		parts := strings.Split(fwd, ";")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if strings.HasPrefix(strings.ToLower(p), "for=") {
				v := strings.TrimSpace(p[4:])
				v = strings.Trim(v, "\"")
				// Could be: ip, [ip]:port, ip:port
				v = strings.TrimPrefix(v, "[")
				v = strings.TrimSuffix(v, "]")
				if host, _, err := net.SplitHostPort(v); err == nil {
					return host
				}
				// Might be just an IP without port
				if ip := net.ParseIP(v); ip != nil {
					return v
				}
				return v
			}
		}
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// First IP is the original client.
		first := strings.TrimSpace(strings.Split(xff, ",")[0])
		first = strings.Trim(first, "\"")
		first = strings.TrimPrefix(first, "[")
		first = strings.TrimSuffix(first, "]")
		if host, _, err := net.SplitHostPort(first); err == nil {
			return host
		}
		return first
	}

	if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
		if host, _, err := net.SplitHostPort(xrip); err == nil {
			return host
		}
		return xrip
	}

	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
