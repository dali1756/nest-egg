-- +goose Up
CREATE TABLE password_reset_tokens (
    id         BIGSERIAL   PRIMARY KEY,
    token_hash BYTEA       NOT NULL UNIQUE,
    user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX password_reset_tokens_user_id_idx ON password_reset_tokens (user_id);

-- +goose Down
DROP TABLE password_reset_tokens;
