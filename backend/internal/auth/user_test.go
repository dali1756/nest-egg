package auth

import (
	"testing"

	"github.com/aarondl/authboss/v3"
)

var _ authboss.AuthableUser = (*User)(nil)

func TestUserPutPIDNormalizesEmail(t *testing.T) {
	tests := map[string]string{
		"Me@Example.com":       "me@example.com",
		"  me@example.com \t ": "me@example.com",
		"me@example.com":       "me@example.com",
	}
	for in, want := range tests {
		u := &User{}
		u.PutPID(in)
		if got := u.GetPID(); got != want {
			t.Errorf("PutPID(%q) then GetPID() = %q, want %q", in, got, want)
		}
	}
}
