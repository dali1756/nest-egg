package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/aarondl/authboss/v3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"nest-egg/backend/internal/platform/db/dbgen"
)

type Repository struct {
	q *dbgen.Queries
}

func NewRepository(db dbgen.DBTX) *Repository {
	return &Repository{q: dbgen.New(db)}
}

func (r *Repository) New(ctx context.Context) authboss.User {
	return &User{}
}

func (r *Repository) Load(ctx context.Context, key string) (authboss.User, error) {
	row, err := r.q.GetUserByEmail(ctx, normalizeEmail(key))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, authboss.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &User{ID: row.ID, Email: row.Email, Password: row.PasswordHash}, nil
}

func (r *Repository) Create(ctx context.Context, user authboss.User) error {
	u := user.(*User)
	id, err := r.q.CreateUser(ctx, dbgen.CreateUserParams{
		Email:        normalizeEmail(u.Email),
		PasswordHash: u.Password,
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return authboss.ErrUserFound
	}
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	u.ID = id
	return nil
}

func (r *Repository) Save(ctx context.Context, user authboss.User) error {
	u := user.(*User)
	n, err := r.q.UpdateUserPassword(ctx, dbgen.UpdateUserPasswordParams{
		Email:        normalizeEmail(u.Email),
		PasswordHash: u.Password,
	})
	if err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	if n == 0 {
		return authboss.ErrUserNotFound
	}
	return nil
}
