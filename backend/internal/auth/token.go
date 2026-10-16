package auth

import (
	"crypto/rand"
	"crypto/sha256"
)

func newToken() (token string, hash []byte) {
	token = rand.Text()
	return token, hashToken(token)
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
