-- name: InsertStageEvent :one
INSERT INTO stage_events (application_id, from_stage, to_stage, detected_via, source_email_id, confidence, note)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListStageEventsForApplication :many
SELECT * FROM stage_events WHERE application_id = $1 ORDER BY occurred_at ASC;
