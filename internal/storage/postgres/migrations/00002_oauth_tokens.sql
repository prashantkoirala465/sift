-- +goose Up
-- Single-user, self-hosted tool: one Gmail account, one row. The id=1 check
-- makes "singleton table" an enforced invariant rather than a convention
-- someone can accidentally violate with a second INSERT.
CREATE TABLE oauth_tokens (
    id                       SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    provider                 TEXT NOT NULL,
    access_token_encrypted   BYTEA NOT NULL,
    refresh_token_encrypted  BYTEA NOT NULL,
    token_type               TEXT NOT NULL,
    expiry                   TIMESTAMPTZ NOT NULL,
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE oauth_tokens;
