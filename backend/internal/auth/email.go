package auth

import (
	"net/mail"
	"strings"
	"unicode"
)

const maxEmailLen = 254

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validEmail(email string) bool {
	if len(email) > maxEmailLen || strings.IndexFunc(email, func(r rune) bool { return r > unicode.MaxASCII }) >= 0 {
		return false
	}
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email
}
