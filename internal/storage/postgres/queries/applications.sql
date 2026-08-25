-- name: CreateApplication :one
INSERT INTO applications (company, role_title, source, applied_date, current_stage)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetApplication :one
SELECT * FROM applications WHERE id = $1;

-- name: ListApplications :many
SELECT * FROM applications ORDER BY applied_date DESC;

-- name: UpdateApplicationStage :exec
UPDATE applications SET current_stage = $2, updated_at = now() WHERE id = $1;
