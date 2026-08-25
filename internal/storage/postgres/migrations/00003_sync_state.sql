-- +goose Up
-- Same singleton pattern as oauth_tokens. last_history_id is Gmail's
-- History API checkpoint -- stored as text, not a numeric type, because the
-- API itself treats it as an opaque string and Sift only ever round-trips
-- it back to the API, never does arithmetic on it.
CREATE TABLE sync_state (
    id              SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    last_history_id TEXT NOT NULL DEFAULT '',
    last_synced_at  TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO sync_state (id) VALUES (1);

-- +goose Down
DROP TABLE sync_state;
