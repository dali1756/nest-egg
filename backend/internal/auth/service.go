package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	maxFailedLogins      = 3
	sessionIdleTimeout   = 24 * time.Hour
	sessionLifetime      = 7 * 24 * time.Hour
	sessionTouchInterval = time.Minute
	passwordResetTTL     = 30 * time.Minute
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountLocked      = errors.New("account locked")
	ErrEmailTaken         = errors.New("email already registered")
	ErrNoSession          = errors.New("no valid session")
	ErrResetUnavailable   = errors.New("password reset unavailable")
	ErrInvalidResetToken  = errors.New("invalid or expired reset token")
	ErrNotFound           = errors.New("not found")
)

type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string { return "invalid input" }

type User struct {
	ID    int64
	Email string
}

type UserRecord struct {
	ID           int64
	Email        string
	PasswordHash string
}

type NewSession struct {
	TokenHash  []byte
	UserID     int64
	LastSeenAt time.Time
	ExpiresAt  time.Time
}

type Session struct {
	ID         int64
	UserID     int64
	Email      string
	LastSeenAt time.Time
	ExpiresAt  time.Time
}

type Store interface {
	CreateUser(ctx context.Context, email, passwordHash string) (int64, error)
	UserByEmail(ctx context.Context, email string) (UserRecord, error)
	ReserveLoginAttempt(ctx context.Context, userID int64, maxFailures int) (bool, error)
	LockIfFailuresExhausted(ctx context.Context, userID int64, maxFailures int) (bool, error)
	ClearLoginFailures(ctx context.Context, userID int64) error

	CreateSession(ctx context.Context, s NewSession) error
	SessionByTokenHash(ctx context.Context, tokenHash []byte) (Session, error)
	TouchSession(ctx context.Context, id int64, at time.Time) error

	DeleteSession(ctx context.Context, tokenHash []byte) (int64, error)
	DeleteStaleSessions(ctx context.Context, userID int64, now, idleBefore time.Time) error

	CreatePasswordReset(ctx context.Context, userID int64, tokenHash []byte, expiresAt time.Time) error

	ResetPassword(ctx context.Context, tokenHash []byte, passwordHash string, now time.Time) (int64, error)
}

type Mailer interface {
	SendPasswordReset(ctx context.Context, to, token string) error
}

type Config struct {
	BcryptCost int
	Now        func() time.Time
	Mailer     Mailer
}

type Service struct {
	store  Store
	hasher *passwordHasher
	now    func() time.Time
	mailer Mailer
	log    *slog.Logger
}

func NewService(store Store, cfg Config, log *slog.Logger) (*Service, error) {
	cost := cfg.BcryptCost
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	hasher, err := newPasswordHasher(cost)
	if err != nil {
		return nil, fmt.Errorf("create password hasher: %w", err)
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, hasher: hasher, now: now, mailer: cfg.Mailer, log: log}, nil
}

type RegisterInput struct {
	Email           string
	Password        string
	ConfirmPassword string
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (User, string, error) {
	email := normalizeEmail(in.Email)
	fields := map[string]string{}
	if !validEmail(email) {
		fields["email"] = "must be a valid email address"
	}
	if err := validatePassword(in.Password); err != nil {
		fields["password"] = err.Error()
	}
	if in.ConfirmPassword != in.Password {
		fields["confirm_password"] = "does not match password"
	}
	if len(fields) > 0 {
		return User{}, "", &ValidationError{Fields: fields}
	}

	hash, err := s.hasher.hash(in.Password)
	if err != nil {
		return User{}, "", fmt.Errorf("hash password: %w", err)
	}
	id, err := s.store.CreateUser(ctx, email, hash)
	if errors.Is(err, ErrEmailTaken) {
		return User{}, "", ErrEmailTaken
	}
	if err != nil {
		return User{}, "", fmt.Errorf("create user: %w", err)
	}
	token, err := s.startSession(ctx, id)
	if err != nil {
		return User{}, "", err
	}
	s.log.InfoContext(ctx, "user registered", "user_id", id)
	return User{ID: id, Email: email}, token, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (User, string, error) {
	if password == "" || len(password) > maxBcryptBytes {
		return User{}, "", ErrInvalidCredentials
	}
	u, err := s.store.UserByEmail(ctx, normalizeEmail(email))
	if errors.Is(err, ErrNotFound) {
		s.hasher.waste(password)
		s.log.InfoContext(ctx, "login failed", "reason", "unknown_email")
		return User{}, "", ErrInvalidCredentials
	}
	if err != nil {
		return User{}, "", fmt.Errorf("get user: %w", err)
	}

	reserved, err := s.store.ReserveLoginAttempt(ctx, u.ID, maxFailedLogins)
	if err != nil {
		return User{}, "", fmt.Errorf("reserve login attempt: %w", err)
	}
	if !reserved {
		s.log.InfoContext(ctx, "login rejected", "reason", "locked", "user_id", u.ID)
		return User{}, "", ErrAccountLocked
	}

	if !s.hasher.matches(u.PasswordHash, password) {
		locked, err := s.store.LockIfFailuresExhausted(ctx, u.ID, maxFailedLogins)
		if err != nil {
			return User{}, "", fmt.Errorf("lock account: %w", err)
		}
		if locked {
			s.log.WarnContext(ctx, "account locked", "user_id", u.ID, "failed_logins", maxFailedLogins)
			return User{}, "", ErrAccountLocked
		}
		s.log.InfoContext(ctx, "login failed", "reason", "wrong_password", "user_id", u.ID)
		return User{}, "", ErrInvalidCredentials
	}

	if err := s.store.ClearLoginFailures(ctx, u.ID); err != nil {
		return User{}, "", fmt.Errorf("clear login failures: %w", err)
	}
	token, err := s.startSession(ctx, u.ID)
	if err != nil {
		return User{}, "", err
	}
	s.log.InfoContext(ctx, "login succeeded", "user_id", u.ID)
	return User{ID: u.ID, Email: u.Email}, token, nil
}

func (s *Service) startSession(ctx context.Context, userID int64) (string, error) {
	now := s.now()
	if err := s.store.DeleteStaleSessions(ctx, userID, now, now.Add(-sessionIdleTimeout)); err != nil {
		return "", fmt.Errorf("delete stale sessions: %w", err)
	}
	token, hash := newToken()
	err := s.store.CreateSession(ctx, NewSession{
		TokenHash:  hash,
		UserID:     userID,
		LastSeenAt: now,
		ExpiresAt:  now.Add(sessionLifetime),
	})
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return token, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (User, error) {
	if token == "" {
		return User{}, ErrNoSession
	}
	hash := hashToken(token)
	sess, err := s.store.SessionByTokenHash(ctx, hash)
	if errors.Is(err, ErrNotFound) {
		return User{}, ErrNoSession
	}
	if err != nil {
		return User{}, fmt.Errorf("get session: %w", err)
	}

	now := s.now()
	if reason := expiredReason(sess, now); reason != "" {
		if _, err := s.store.DeleteSession(ctx, hash); err != nil && !errors.Is(err, ErrNotFound) {
			return User{}, fmt.Errorf("delete expired session: %w", err)
		}
		s.log.InfoContext(ctx, "session expired", "reason", reason, "user_id", sess.UserID)
		return User{}, ErrNoSession
	}
	if now.Sub(sess.LastSeenAt) >= sessionTouchInterval {
		if err := s.store.TouchSession(ctx, sess.ID, now); err != nil {
			return User{}, fmt.Errorf("touch session: %w", err)
		}
	}
	return User{ID: sess.UserID, Email: sess.Email}, nil
}

func expiredReason(sess Session, now time.Time) string {
	switch {
	case !now.Before(sess.ExpiresAt):
		return "lifetime"
	case now.Sub(sess.LastSeenAt) >= sessionIdleTimeout:
		return "idle"
	}
	return ""
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	userID, err := s.store.DeleteSession(ctx, hashToken(token))
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	s.log.InfoContext(ctx, "logout", "user_id", userID)
	return nil
}

func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	if s.mailer == nil {
		return ErrResetUnavailable
	}
	u, err := s.store.UserByEmail(ctx, normalizeEmail(email))
	if errors.Is(err, ErrNotFound) {
		s.log.InfoContext(ctx, "password reset requested", "reason", "unknown_email")
		return nil
	}
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}

	token, hash := newToken()
	if err := s.store.CreatePasswordReset(ctx, u.ID, hash, s.now().Add(passwordResetTTL)); err != nil {
		return fmt.Errorf("create reset token: %w", err)
	}
	if err := s.mailer.SendPasswordReset(ctx, u.Email, token); err != nil {
		return fmt.Errorf("send reset email: %w", err)
	}
	s.log.InfoContext(ctx, "password reset requested", "user_id", u.ID)
	return nil
}

type ResetPasswordInput struct {
	Token           string
	Password        string
	ConfirmPassword string
}

// ResetPassword 成功後所有裝置都會被登出，要用新密碼重新登入。
func (s *Service) ResetPassword(ctx context.Context, in ResetPasswordInput) error {
	fields := map[string]string{}
	if err := validatePassword(in.Password); err != nil {
		fields["password"] = err.Error()
	}
	if in.ConfirmPassword != in.Password {
		fields["confirm_password"] = "does not match password"
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	if in.Token == "" {
		return ErrInvalidResetToken
	}

	hash, err := s.hasher.hash(in.Password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	userID, err := s.store.ResetPassword(ctx, hashToken(in.Token), hash, s.now())
	if errors.Is(err, ErrNotFound) {
		s.log.InfoContext(ctx, "password reset failed", "reason", "invalid_token")
		return ErrInvalidResetToken
	}
	if err != nil {
		return fmt.Errorf("reset password: %w", err)
	}
	s.log.InfoContext(ctx, "password reset", "user_id", userID)
	return nil
}
