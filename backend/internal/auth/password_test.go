package auth

import (
	"strings"
	"testing"
)

func TestValidatePassword(t *testing.T) {
	valid := []string{
		"Passw0rd!",
		"Aa1!aaaa",
		"Aa1" + strings.Repeat("~", 61),
		"Aa1`{}|\\\"'",
	}
	for _, p := range valid {
		if err := validatePassword(p); err != nil {
			t.Errorf("validatePassword(%q) = %v, want nil", p, err)
		}
	}

	invalid := map[string]string{
		"":                               "at least 8",
		"Pa0rd!":                         "at least 8",
		"Aa1!" + strings.Repeat("a", 61): "at most 64",
		"Passw0rd":                       "symbol",
		"passw0rd!":                      "uppercase",
		"PASSW0RD!":                      "lowercase",
		"Password!":                      "digit",
		"Pass w0rd!":                     "without spaces",
		"Passw0rd!\t":                    "without spaces",
		"Pässw0rd!":                      "without spaces",
		"密碼Passw0rd!":                    "without spaces",
	}
	for p, want := range invalid {
		err := validatePassword(p)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("validatePassword(%q) = %v, want error containing %q", p, err, want)
		}
	}
}

func TestPasswordHasher(t *testing.T) {
	h, err := newPasswordHasher(4)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := h.hash("Passw0rd!")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "Passw0rd!" || !strings.HasPrefix(hash, "$2") {
		t.Fatalf("hash = %q, want a bcrypt hash", hash)
	}
	if !h.matches(hash, "Passw0rd!") {
		t.Error("matches(correct password) = false")
	}
	if h.matches(hash, "Passw0rd?") {
		t.Error("matches(wrong password) = true")
	}
}
