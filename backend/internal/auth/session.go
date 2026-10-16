package auth

import (
	"crypto/hkdf"
	"crypto/sha256"
	"fmt"
	"net/http"

	abclientstate "github.com/aarondl/authboss-clientstate"
	"github.com/aarondl/authboss/v3"
	"github.com/gorilla/sessions"
)

const (
	sessionCookieName = "nest_egg_session"
	sessionMaxAge     = 7 * 24 * 60 * 60 // 7 天（s）
)

func newSessionStorer(secret []byte, secure bool) (authboss.ClientStateReadWriter, error) {
	hashKey, err := hkdf.Key(sha256.New, secret, nil, "nest-egg session hmac", 64)
	if err != nil {
		return nil, fmt.Errorf("derive session hash key: %w", err)
	}
	blockKey, err := hkdf.Key(sha256.New, secret, nil, "nest-egg session aes", 32)
	if err != nil {
		return nil, fmt.Errorf("derive session block key: %w", err)
	}

	store := sessions.NewCookieStore(hashKey, blockKey)
	store.MaxAge(sessionMaxAge)
	store.Options.Path = "/"
	store.Options.HttpOnly = true
	store.Options.SameSite = http.SameSiteLaxMode
	store.Options.Secure = secure

	return abclientstate.NewSessionStorerFromExisting(sessionCookieName, store), nil
}
