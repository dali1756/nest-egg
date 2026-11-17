package authtest

import (
	"context"
	"sync"
	"time"

	"nest-egg/backend/internal/auth"
)

type memUser struct {
	rec         auth.UserRecord
	failed      int
	lockCount   int
	lockedUntil time.Time
}

// unlock 和手動解鎖的 SQL 相同：解除鎖定，失敗次數與鎖過幾次都歸零
func (u *memUser) unlock() {
	u.failed = 0
	u.lockCount = 0
	u.lockedUntil = time.Time{}
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

func (m *MemStore) Unlock(email string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u := m.byEmail(email); u != nil {
		u.unlock()
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

func (m *MemStore) ReserveLoginAttempt(_ context.Context, userID int64, limit auth.AttemptLimit) (auth.LoginAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return auth.LoginAttempt{}, m.err
	}
	u := m.users[userID]
	if u == nil {
		return auth.LoginAttempt{}, auth.ErrNotFound
	}
	if u.lockedUntil.After(limit.Now) {
		return auth.LoginAttempt{LockedUntil: u.lockedUntil}, nil
	}
	u.failed++
	if u.failed < limit.MaxFailures {
		u.lockedUntil = time.Time{}
		return auth.LoginAttempt{Reserved: true}, nil
	}
	u.failed = 0
	u.lockedUntil = limit.FirstLockUntil
	if u.lockCount > 0 {
		u.lockedUntil = limit.RepeatLockUntil
	}
	u.lockCount++
	return auth.LoginAttempt{Reserved: true, LockedUntil: u.lockedUntil}, nil
}

func (m *MemStore) ClearLoginFailures(_ context.Context, userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	if u := m.users[userID]; u != nil {
		u.unlock()
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
	m.replacePassword(r.userID, passwordHash)
	return r.userID, nil
}

func (m *MemStore) ChangePassword(_ context.Context, userID int64, passwordHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.replacePassword(userID, passwordHash)
	return nil
}

func (m *MemStore) replacePassword(userID int64, passwordHash string) {
	u := m.users[userID]
	u.rec.PasswordHash = passwordHash
	u.unlock()
	for k, s := range m.sessions {
		if s.UserID == userID {
			delete(m.sessions, k)
		}
	}
	m.deleteResets(userID)
}

func (m *MemStore) UpdatePasswordHash(_ context.Context, userID int64, passwordHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	u := m.users[userID]
	u.rec.PasswordHash = passwordHash
	u.unlock()
	return nil
}

func (m *MemStore) deleteResets(userID int64) {
	for k, r := range m.resets {
		if r.userID == userID {
			delete(m.resets, k)
		}
	}
}
