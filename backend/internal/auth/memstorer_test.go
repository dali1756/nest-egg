package auth

import (
	"context"
	"sync"

	"github.com/aarondl/authboss/v3"
)

type memStorer struct {
	mu      sync.Mutex
	users   map[string]User
	failErr error
}

func newMemStorer() *memStorer { return &memStorer{users: map[string]User{}} }

func (m *memStorer) New(ctx context.Context) authboss.User { return &User{} }

func (m *memStorer) Load(ctx context.Context, key string) (authboss.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failErr != nil {
		return nil, m.failErr
	}
	u, ok := m.users[normalizeEmail(key)]
	if !ok {
		return nil, authboss.ErrUserNotFound
	}
	return &u, nil
}

func (m *memStorer) Save(ctx context.Context, user authboss.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failErr != nil {
		return m.failErr
	}
	u := user.(*User)
	if _, ok := m.users[u.Email]; !ok {
		return authboss.ErrUserNotFound
	}
	m.users[u.Email] = *u
	return nil
}

func (m *memStorer) Create(ctx context.Context, user authboss.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failErr != nil {
		return m.failErr
	}
	u := user.(*User)
	if _, ok := m.users[u.Email]; ok {
		return authboss.ErrUserFound
	}
	u.ID = int64(len(m.users) + 1)
	m.users[u.Email] = *u
	return nil
}

func (m *memStorer) get(email string) (User, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[email]
	return u, ok
}

func (m *memStorer) setFail(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failErr = err
}
