package auth

import (
	"net/http"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestPasswordIsStoredAsBcryptHashOnly(t *testing.T) {
	e := newTestEnv(t)
	c := e.client(t)

	responses := []response{
		e.register(t, c, "me@example.com"),
		e.do(t, c, http.MethodGet, "/api/auth/me", nil),
		e.do(t, c, http.MethodPost, "/api/auth/logout", nil),
		e.do(t, c, http.MethodPost, "/api/auth/login", loginBody("me@example.com", "Wr0ngpass!")),
		e.do(t, c, http.MethodPost, "/api/auth/login", loginBody("me@example.com", goodPassword)),
	}

	u, ok := e.storer.get("me@example.com")
	if !ok {
		t.Fatal("user not stored")
	}
	if u.Password == goodPassword || !strings.HasPrefix(u.Password, "$2") {
		t.Fatalf("stored password = %q, want a bcrypt hash", u.Password)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(goodPassword)); err != nil {
		t.Errorf("stored hash does not match the password: %v", err)
	}

	secrets := []string{goodPassword, "Wr0ngpass!", u.Password}
	for i, r := range responses {
		for _, s := range secrets {
			if strings.Contains(r.body, s) {
				t.Errorf("response %d contains secret %q: %s", i, s, r.body)
			}
		}
	}
	for _, s := range secrets {
		if strings.Contains(e.logs.String(), s) {
			t.Errorf("logs contain secret %q", s)
		}
	}
}
