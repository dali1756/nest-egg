package auth

import (
	"strings"
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	if got := normalizeEmail("  Me@Example.COM "); got != "me@example.com" {
		t.Errorf("normalizeEmail = %q, want me@example.com", got)
	}
}

func TestValidEmail(t *testing.T) {
	tests := map[string]bool{
		"me@example.com":                          true,
		"first.last+tag@mail.example.tw":          true,
		"not-an-email":                            false,
		"me@":                                     false,
		"Me <me@example.com>":                     false,
		"me@example.com, you@example.com":         false,
		"ｍｅ@example.com":                          false,
		"me@exämple.com":                          false,
		strings.Repeat("a", 243) + "@example.com": false,
	}
	for email, want := range tests {
		if got := validEmail(email); got != want {
			t.Errorf("validEmail(%q) = %v, want %v", email, got, want)
		}
	}
}
