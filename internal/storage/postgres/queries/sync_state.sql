-- name: GetSyncState :one
SELECT * FROM sync_state WHERE id = 1;

-- name: UpdateSyncState :exec
UPDATE sync_state SET last_history_id = $1, last_synced_at = $2, updated_at = now() WHERE id = 1;
