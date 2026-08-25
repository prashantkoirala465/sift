-- name: InsertEmailMessage :one
INSERT INTO email_messages (gmail_message_id, gmail_thread_id, from_address, from_domain, subject, received_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetEmailMessageByGmailID :one
SELECT * FROM email_messages WHERE gmail_message_id = $1;
