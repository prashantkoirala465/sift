-- +goose Up
CREATE TABLE applications (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company       TEXT NOT NULL,
    role_title    TEXT NOT NULL,
    source        TEXT NOT NULL CHECK (source IN ('linkedin', 'referral', 'company_site', 'job_board', 'other')),
    applied_date  DATE NOT NULL,
    current_stage TEXT NOT NULL CHECK (current_stage IN ('applied', 'screening', 'interview', 'offer', 'accepted', 'declined', 'rejected', 'withdrawn')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_applications_current_stage ON applications (current_stage);

CREATE TABLE email_messages (
    id                         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    gmail_message_id           TEXT NOT NULL UNIQUE,
    gmail_thread_id            TEXT NOT NULL,
    from_address               TEXT NOT NULL,
    from_domain                TEXT NOT NULL,
    subject                    TEXT NOT NULL,
    received_at                TIMESTAMPTZ NOT NULL,
    classified_label           TEXT CHECK (classified_label IN ('confirmation', 'rejection', 'interview', 'offer', 'assessment', 'other', 'unclassified')),
    classification_confidence  DOUBLE PRECISION CHECK (classification_confidence IS NULL OR classification_confidence BETWEEN 0 AND 1),
    classification_source      TEXT CHECK (classification_source IN ('rule', 'llm')),
    matched_application_id     UUID REFERENCES applications (id) ON DELETE SET NULL,
    match_confidence           DOUBLE PRECISION CHECK (match_confidence IS NULL OR match_confidence BETWEEN 0 AND 1),
    review_status               TEXT NOT NULL DEFAULT 'pending' CHECK (review_status IN ('pending', 'matched', 'ignored')),
    processed_at                TIMESTAMPTZ,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_email_messages_matched_application_id ON email_messages (matched_application_id);
CREATE INDEX idx_email_messages_gmail_thread_id ON email_messages (gmail_thread_id);
CREATE INDEX idx_email_messages_review_status_pending ON email_messages (review_status) WHERE review_status = 'pending';

CREATE TABLE stage_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id  UUID NOT NULL REFERENCES applications (id) ON DELETE CASCADE,
    from_stage      TEXT NOT NULL CHECK (from_stage IN ('applied', 'screening', 'interview', 'offer', 'accepted', 'declined', 'rejected', 'withdrawn')),
    to_stage        TEXT NOT NULL CHECK (to_stage IN ('applied', 'screening', 'interview', 'offer', 'accepted', 'declined', 'rejected', 'withdrawn')),
    detected_via    TEXT NOT NULL CHECK (detected_via IN ('email_auto', 'manual')),
    source_email_id UUID REFERENCES email_messages (id) ON DELETE SET NULL,
    confidence      DOUBLE PRECISION CHECK (confidence IS NULL OR confidence BETWEEN 0 AND 1),
    note            TEXT NOT NULL DEFAULT '',
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_stage_events_application_id ON stage_events (application_id, occurred_at);

-- +goose Down
DROP TABLE stage_events;
DROP TABLE email_messages;
DROP TABLE applications;
