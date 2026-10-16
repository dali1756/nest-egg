package auth

import (
	"errors"
	"testing"

	"github.com/aarondl/authboss/v3"

	"nest-egg/backend/internal/platform/db/dbtest"
)

var _ authboss.CreatingServerStorer = (*Repository)(nil)

func newTestRepository(t *testing.T) *Repository {
	t.Helper()
	return NewRepository(dbtest.Migrated(t))
}

func createUser(t *testing.T, repo *Repository, email, hash string) {
	t.Helper()
	u := repo.New(t.Context())
	u.(*User).PutPID(email)
	u.(*User).PutPassword(hash)
	if err := repo.Create(t.Context(), u); err != nil {
		t.Fatalf("Create(%s): %v", email, err)
	}
}

func TestRepositoryLoadMissingUser(t *testing.T) {
	repo := newTestRepository(t)

	_, err := repo.Load(t.Context(), "nobody@example.com")
	if !errors.Is(err, authboss.ErrUserNotFound) {
		t.Fatalf("Load error = %v, want authboss.ErrUserNotFound", err)
	}
}

func TestRepositoryCreateThenLoadIgnoresCase(t *testing.T) {
	repo := newTestRepository(t)
	createUser(t, repo, "me@example.com", "$2a$10$hash")

	got, err := repo.Load(t.Context(), "  ME@Example.com ")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	u := got.(*User)
	if u.ID == 0 || u.Email != "me@example.com" || u.Password != "$2a$10$hash" {
		t.Errorf("Load = %+v, want ID>0, email me@example.com, password $2a$10$hash", u)
	}
}

func TestRepositoryCreateDuplicate(t *testing.T) {
	repo := newTestRepository(t)
	createUser(t, repo, "me@example.com", "h1")

	u := repo.New(t.Context())
	u.(*User).PutPID("ME@example.com")
	u.(*User).PutPassword("h2")
	if err := repo.Create(t.Context(), u); !errors.Is(err, authboss.ErrUserFound) {
		t.Fatalf("Create duplicate error = %v, want authboss.ErrUserFound", err)
	}
}

func TestRepositorySaveUpdatesPassword(t *testing.T) {
	repo := newTestRepository(t)
	createUser(t, repo, "me@example.com", "old")

	u, err := repo.Load(t.Context(), "me@example.com")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	u.(*User).PutPassword("new")
	if err := repo.Save(t.Context(), u); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := repo.Load(t.Context(), "me@example.com")
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if pw := got.(*User).Password; pw != "new" {
		t.Errorf("password after Save = %q, want %q", pw, "new")
	}
}

func TestRepositorySaveMissingUser(t *testing.T) {
	repo := newTestRepository(t)

	err := repo.Save(t.Context(), &User{Email: "nobody@example.com", Password: "x"})
	if !errors.Is(err, authboss.ErrUserNotFound) {
		t.Fatalf("Save error = %v, want authboss.ErrUserNotFound", err)
	}
}
