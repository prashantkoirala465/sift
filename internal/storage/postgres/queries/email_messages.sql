-- name: InsertEmailMessageIfNew :one
INSERT INTO email_messages (gmail_message_id, gmail_thread_id, from_address, from_domain, subject, snippet, received_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (gmail_message_id) DO NOTHING
RETURNING *;

-- name: GetEmailMessageByGmailID :one
SELECT * FROM email_messages WHERE gmail_message_id = $1;

-- name: SetEmailClassification :exec
UPDATE email_messages
SET classified_label = $2,
    classification_confidence = $3,
    classification_source = $4,
    processed_at = now()
WHERE id = $1;

-- name: FindApplicationIDByThreadID :one
SELECT matched_application_id FROM email_messages
WHERE gmail_thread_id = $1 AND matched_application_id IS NOT NULL
ORDER BY received_at DESC
LIMIT 1;

-- name: ListDistinctMatchedApplicationsByDomain :many
SELECT DISTINCT matched_application_id FROM email_messages
WHERE from_domain = $1 AND matched_application_id IS NOT NULL;

-- name: SetEmailMatch :exec
UPDATE email_messages
SET matched_application_id = $2,
    match_confidence = $3,
    review_status = $4
WHERE id = $1;

-- name: SetEmailReviewStatus :exec
UPDATE email_messages SET review_status = $2 WHERE id = $1;

-- name: GetEmailMessage :one
SELECT * FROM email_messages WHERE id = $1;

-- name: ListEmailMessagesByReviewStatus :many
SELECT * FROM email_messages WHERE review_status = $1 ORDER BY received_at DESC;
