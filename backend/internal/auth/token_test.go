package auth

import (
	"bytes"
	"crypto/sha256"
	"testing"
)

func TestNewToken(t *testing.T) {
	token, hash := newToken()
	other, _ := newToken()
	if len(token) != 26 || token == other {
		t.Errorf("tokens %q, %q: want two different 26-character tokens", token, other)
	}
	if want := sha256.Sum256([]byte(token)); !bytes.Equal(hash, want[:]) {
		t.Errorf("hash = %x, want sha256(token) %x", hash, want)
	}
}
