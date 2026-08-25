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
