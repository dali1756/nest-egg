package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"nest-egg/backend/internal/platform/db/dbgen"
)

type DB interface {
	dbgen.DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

type Repository struct {
	db DB
	q  *dbgen.Queries
}

func NewRepository(db DB) *Repository {
	return &Repository{db: db, q: dbgen.New(db)}
}

func (r *Repository) inTx(ctx context.Context, fn func(q *dbgen.Queries) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := fn(r.q.WithTx(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (r *Repository) CreateUser(ctx context.Context, email, passwordHash string) (int64, error) {
	id, err := r.q.CreateUser(ctx, dbgen.CreateUserParams{Email: email, PasswordHash: passwordHash})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return 0, ErrEmailTaken
	}
	if err != nil {
		return 0, fmt.Errorf("insert user: %w", err)
	}
	return id, nil
}

func (r *Repository) UserByEmail(ctx context.Context, email string) (UserRecord, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return UserRecord{}, ErrNotFound
	}
	if err != nil {
		return UserRecord{}, fmt.Errorf("select user: %w", err)
	}
	return UserRecord{ID: row.ID, Email: row.Email, PasswordHash: row.PasswordHash}, nil
}

func (r *Repository) ReserveLoginAttempt(ctx context.Context, userID int64, maxFailures int) (bool, error) {
	n, err := r.q.ReserveLoginAttempt(ctx, dbgen.ReserveLoginAttemptParams{ID: userID, MaxFailures: int16(maxFailures)})
	if err != nil {
		return false, fmt.Errorf("update failed_login_count: %w", err)
	}
	return n == 1, nil
}

func (r *Repository) LockIfFailuresExhausted(ctx context.Context, userID int64, maxFailures int) (bool, error) {
	n, err := r.q.LockIfFailuresExhausted(ctx, dbgen.LockIfFailuresExhaustedParams{ID: userID, MaxFailures: int16(maxFailures)})
	if err != nil {
		return false, fmt.Errorf("update locked: %w", err)
	}
	return n == 1, nil
}

func (r *Repository) ClearLoginFailures(ctx context.Context, userID int64) error {
	if err := r.q.ClearLoginFailures(ctx, userID); err != nil {
		return fmt.Errorf("reset failed_login_count: %w", err)
	}
	return nil
}

func (r *Repository) CreateSession(ctx context.Context, s NewSession) error {
	err := r.q.CreateSession(ctx, dbgen.CreateSessionParams{
		TokenHash:  s.TokenHash,
		UserID:     s.UserID,
		LastSeenAt: s.LastSeenAt,
		ExpiresAt:  s.ExpiresAt,
	})
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

func (r *Repository) SessionByTokenHash(ctx context.Context, tokenHash []byte) (Session, error) {
	row, err := r.q.GetSessionByTokenHash(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("select session: %w", err)
	}
	return Session{
		ID:         row.ID,
		UserID:     row.UserID,
		Email:      row.Email,
		LastSeenAt: row.LastSeenAt,
		ExpiresAt:  row.ExpiresAt,
	}, nil
}

func (r *Repository) TouchSession(ctx context.Context, id int64, at time.Time) error {
	if err := r.q.TouchSession(ctx, dbgen.TouchSessionParams{ID: id, LastSeenAt: at}); err != nil {
		return fmt.Errorf("update last_seen_at: %w", err)
	}
	return nil
}

func (r *Repository) DeleteSession(ctx context.Context, tokenHash []byte) (int64, error) {
	userID, err := r.q.DeleteSession(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("delete session: %w", err)
	}
	return userID, nil
}

func (r *Repository) DeleteStaleSessions(ctx context.Context, userID int64, now, idleBefore time.Time) error {
	err := r.q.DeleteStaleSessions(ctx, dbgen.DeleteStaleSessionsParams{UserID: userID, Now: now, IdleBefore: idleBefore})
	if err != nil {
		return fmt.Errorf("delete stale sessions: %w", err)
	}
	return nil
}

func (r *Repository) CreatePasswordReset(ctx context.Context, userID int64, tokenHash []byte, expiresAt time.Time) error {
	return r.inTx(ctx, func(q *dbgen.Queries) error {
		if err := q.DeleteUserPasswordResetTokens(ctx, userID); err != nil {
			return fmt.Errorf("delete old reset tokens: %w", err)
		}
		err := q.CreatePasswordResetToken(ctx, dbgen.CreatePasswordResetTokenParams{TokenHash: tokenHash, UserID: userID, ExpiresAt: expiresAt})
		if err != nil {
			return fmt.Errorf("insert reset token: %w", err)
		}
		return nil
	})
}

func (r *Repository) ResetPassword(ctx context.Context, tokenHash []byte, passwordHash string, now time.Time) (int64, error) {
	var userID int64
	err := r.inTx(ctx, func(q *dbgen.Queries) error {
		row, err := q.ConsumePasswordResetToken(ctx, tokenHash)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !row.ExpiresAt.After(now)) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("delete reset token: %w", err)
		}
		userID = row.UserID
		return replacePassword(ctx, q, userID, passwordHash)
	})
	if err != nil {
		return 0, err
	}
	return userID, nil
}

func (r *Repository) ChangePassword(ctx context.Context, userID int64, passwordHash string) error {
	return r.inTx(ctx, func(q *dbgen.Queries) error {
		return replacePassword(ctx, q, userID, passwordHash)
	})
}

func replacePassword(ctx context.Context, q *dbgen.Queries, userID int64, passwordHash string) error {
	if err := q.UpdatePasswordHash(ctx, dbgen.UpdatePasswordHashParams{ID: userID, PasswordHash: passwordHash}); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if err := q.DeleteUserSessions(ctx, userID); err != nil {
		return fmt.Errorf("delete sessions: %w", err)
	}
	if err := q.DeleteUserPasswordResetTokens(ctx, userID); err != nil {
		return fmt.Errorf("delete reset tokens: %w", err)
	}
	return nil
}

func (r *Repository) UpdatePasswordHash(ctx context.Context, userID int64, passwordHash string) error {
	if err := r.q.UpdatePasswordHash(ctx, dbgen.UpdatePasswordHashParams{ID: userID, PasswordHash: passwordHash}); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}
