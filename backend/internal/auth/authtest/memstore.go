package authtest

import (
	"context"
	"sync"
	"time"

	"nest-egg/backend/internal/auth"
)

type memUser struct {
	rec    auth.UserRecord
	failed int
	locked bool
}

type memReset struct {
	userID    int64
	expiresAt time.Time
}

type MemStore struct {
	mu       sync.Mutex
	nextID   int64
	users    map[int64]*memUser
	sessions map[string]*auth.Session
	resets   map[string]memReset
	err      error
}

var _ auth.Store = (*MemStore)(nil)

func NewMemStore() *MemStore {
	return &MemStore{users: map[int64]*memUser{}, sessions: map[string]*auth.Session{}, resets: map[string]memReset{}}
}

func (m *MemStore) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func (m *MemStore) Locked(email string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.byEmail(email)
	return u != nil && u.locked
}

func (m *MemStore) Unlock(email string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u := m.byEmail(email); u != nil {
		u.locked = false
	}
}

func (m *MemStore) PasswordHash(email string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u := m.byEmail(email); u != nil {
		return u.rec.PasswordHash
	}
	return ""
}

func (m *MemStore) SessionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}

func (m *MemStore) byEmail(email string) *memUser {
	for _, u := range m.users {
		if u.rec.Email == email {
			return u
		}
	}
	return nil
}

func (m *MemStore) CreateUser(_ context.Context, email, passwordHash string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return 0, m.err
	}
	if m.byEmail(email) != nil {
		return 0, auth.ErrEmailTaken
	}
	m.nextID++
	m.users[m.nextID] = &memUser{rec: auth.UserRecord{ID: m.nextID, Email: email, PasswordHash: passwordHash}}
	return m.nextID, nil
}

func (m *MemStore) UserByEmail(_ context.Context, email string) (auth.UserRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return auth.UserRecord{}, m.err
	}
	u := m.byEmail(email)
	if u == nil {
		return auth.UserRecord{}, auth.ErrNotFound
	}
	return u.rec, nil
}

func (m *MemStore) ReserveLoginAttempt(_ context.Context, userID int64, maxFailures int) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return false, m.err
	}
	u := m.users[userID]
	if u == nil || u.locked || u.failed >= maxFailures {
		return false, nil
	}
	u.failed++
	return true, nil
}

func (m *MemStore) LockIfFailuresExhausted(_ context.Context, userID int64, maxFailures int) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return false, m.err
	}
	u := m.users[userID]
	if u == nil || u.failed < maxFailures {
		return false, nil
	}
	u.locked = true
	u.failed = 0
	return true, nil
}

func (m *MemStore) ClearLoginFailures(_ context.Context, userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	if u := m.users[userID]; u != nil {
		u.failed = 0
	}
	return nil
}

func (m *MemStore) CreateSession(_ context.Context, s auth.NewSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.nextID++
	m.sessions[string(s.TokenHash)] = &auth.Session{
		ID:         m.nextID,
		UserID:     s.UserID,
		LastSeenAt: s.LastSeenAt,
		ExpiresAt:  s.ExpiresAt,
	}
	return nil
}

func (m *MemStore) SessionByTokenHash(_ context.Context, tokenHash []byte) (auth.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return auth.Session{}, m.err
	}
	s := m.sessions[string(tokenHash)]
	if s == nil {
		return auth.Session{}, auth.ErrNotFound
	}
	out := *s
	out.Email = m.users[s.UserID].rec.Email
	return out, nil
}

func (m *MemStore) TouchSession(_ context.Context, id int64, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	for _, s := range m.sessions {
		if s.ID == id {
			s.LastSeenAt = at
		}
	}
	return nil
}

func (m *MemStore) DeleteSession(_ context.Context, tokenHash []byte) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return 0, m.err
	}
	s := m.sessions[string(tokenHash)]
	if s == nil {
		return 0, auth.ErrNotFound
	}
	delete(m.sessions, string(tokenHash))
	return s.UserID, nil
}

func (m *MemStore) DeleteStaleSessions(_ context.Context, userID int64, now, idleBefore time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	for k, s := range m.sessions {
		if s.UserID == userID && (!s.ExpiresAt.After(now) || !s.LastSeenAt.After(idleBefore)) {
			delete(m.sessions, k)
		}
	}
	return nil
}

func (m *MemStore) CreatePasswordReset(_ context.Context, userID int64, tokenHash []byte, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.deleteResets(userID)
	m.resets[string(tokenHash)] = memReset{userID: userID, expiresAt: expiresAt}
	return nil
}

func (m *MemStore) ResetPassword(_ context.Context, tokenHash []byte, passwordHash string, now time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return 0, m.err
	}
	r, ok := m.resets[string(tokenHash)]
	if !ok || !r.expiresAt.After(now) {
		return 0, auth.ErrNotFound
	}
	u := m.users[r.userID]
	u.rec.PasswordHash = passwordHash
	u.failed = 0
	for k, s := range m.sessions {
		if s.UserID == r.userID {
			delete(m.sessions, k)
		}
	}
	m.deleteResets(r.userID)
	return r.userID, nil
}

func (m *MemStore) deleteResets(userID int64) {
	for k, r := range m.resets {
		if r.userID == userID {
			delete(m.resets, k)
		}
	}
}
