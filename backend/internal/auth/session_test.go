package auth

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aarondl/authboss/v3"
)

var (
	testSecret  = bytes.Repeat([]byte{0x01}, 32)
	otherSecret = bytes.Repeat([]byte{0x02}, 32)
)

func newTestSessionStorer(t *testing.T, secret []byte, secure bool) authboss.ClientStateReadWriter {
	t.Helper()
	s, err := newSessionStorer(secret, secure)
	if err != nil {
		t.Fatalf("newSessionStorer: %v", err)
	}
	return s
}

func loginCookie(t *testing.T, s authboss.ClientStateReadWriter) *http.Cookie {
	t.Helper()
	state, err := s.ReadState(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	rec := httptest.NewRecorder()
	ev := []authboss.ClientStateEvent{{Kind: authboss.ClientStateEventPut, Key: authboss.SessionKey, Value: "me@example.com"}}
	if err := s.WriteState(rec, state, ev); err != nil {
		t.Fatalf("WriteState: %v", err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}
	return cookies[0]
}

func sessionPID(t *testing.T, s authboss.ClientStateReadWriter, c *http.Cookie) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(c)
	state, err := s.ReadState(req)
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	pid, _ := state.Get(authboss.SessionKey)
	return pid
}

func TestSessionCookieAttributes(t *testing.T) {
	for _, secure := range []bool{true, false} {
		c := loginCookie(t, newTestSessionStorer(t, testSecret, secure))

		if c.Name != "nest_egg_session" {
			t.Errorf("Name = %q, want nest_egg_session", c.Name)
		}
		if !c.HttpOnly {
			t.Error("HttpOnly = false, want true")
		}
		if c.SameSite != http.SameSiteLaxMode {
			t.Errorf("SameSite = %v, want Lax", c.SameSite)
		}
		if c.Path != "/" {
			t.Errorf("Path = %q, want /", c.Path)
		}
		if c.MaxAge != 7*24*60*60 {
			t.Errorf("MaxAge = %d, want %d", c.MaxAge, 7*24*60*60)
		}
		if c.Secure != secure {
			t.Errorf("Secure = %v, want %v", c.Secure, secure)
		}
	}
}

func TestSessionRoundTrip(t *testing.T) {
	s := newTestSessionStorer(t, testSecret, true)
	c := loginCookie(t, s)

	if pid := sessionPID(t, s, c); pid != "me@example.com" {
		t.Errorf("session pid = %q, want me@example.com", pid)
	}
}

func TestSessionCookieIsEncrypted(t *testing.T) {
	c := loginCookie(t, newTestSessionStorer(t, testSecret, true))

	outer, err := base64.URLEncoding.DecodeString(c.Value)
	if err != nil {
		t.Fatalf("decode cookie: %v", err)
	}
	parts := bytes.SplitN(outer, []byte("|"), 3)
	if len(parts) != 3 {
		t.Fatalf("cookie has %d parts, want 3", len(parts))
	}
	payload, err := base64.URLEncoding.DecodeString(string(parts[1]))
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if bytes.Contains(payload, []byte("me@example.com")) {
		t.Error("cookie payload contains the plain email; want it encrypted")
	}
}

func TestSessionRejectsTamperedCookie(t *testing.T) {
	s := newTestSessionStorer(t, testSecret, true)
	c := loginCookie(t, s)
	b := []byte(c.Value)
	b[len(b)/2] ^= 0x01
	c.Value = string(b)

	if pid := sessionPID(t, s, c); pid != "" {
		t.Errorf("tampered cookie gave pid %q, want empty", pid)
	}
}

func TestSessionRejectsCookieFromOtherSecret(t *testing.T) {
	c := loginCookie(t, newTestSessionStorer(t, otherSecret, true))

	if pid := sessionPID(t, newTestSessionStorer(t, testSecret, true), c); pid != "" {
		t.Errorf("cookie signed with another secret gave pid %q, want empty", pid)
	}
}
