-- +goose Up
-- Gmail's own short plain-text preview. Stored alongside the message both
-- as classifier input and so the review-queue UI has something to show the
-- user beyond a bare subject line.
ALTER TABLE email_messages ADD COLUMN snippet TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE email_messages DROP COLUMN snippet;
