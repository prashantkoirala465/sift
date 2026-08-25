-- name: UpsertOAuthToken :exec
INSERT INTO oauth_tokens (id, provider, access_token_encrypted, refresh_token_encrypted, token_type, expiry, updated_at)
VALUES (1, $1, $2, $3, $4, $5, now())
ON CONFLICT (id) DO UPDATE SET
    provider                = EXCLUDED.provider,
    access_token_encrypted  = EXCLUDED.access_token_encrypted,
    refresh_token_encrypted = EXCLUDED.refresh_token_encrypted,
    token_type              = EXCLUDED.token_type,
    expiry                  = EXCLUDED.expiry,
    updated_at               = now();

-- name: GetOAuthToken :one
SELECT * FROM oauth_tokens WHERE id = 1;
