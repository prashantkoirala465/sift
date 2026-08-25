package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/prashantkoirala465/sift/internal/domain"
	"github.com/prashantkoirala465/sift/internal/storage/postgres/sqlc"
)

type Store struct {
	pool          *pgxpool.Pool
	q             *sqlc.Queries
	encryptionKey []byte
}

// encryptionKey must be security.KeySize bytes; it's used only for the
// secrets in oauth_token.go (see there for why).
func NewStore(pool *pgxpool.Pool, encryptionKey []byte) *Store {
	return &Store{pool: pool, q: sqlc.New(pool), encryptionKey: encryptionKey}
}

func (s *Store) CreateApplication(ctx context.Context, company, roleTitle string, source domain.Source, appliedDate time.Time) (domain.Application, error) {
	row, err := s.q.CreateApplication(ctx, sqlc.CreateApplicationParams{
		Company:      company,
		RoleTitle:    roleTitle,
		Source:       string(source),
		AppliedDate:  pgDate(appliedDate),
		CurrentStage: string(domain.StageApplied),
	})
	if err != nil {
		return domain.Application{}, fmt.Errorf("create application: %w", err)
	}
	return applicationFromRow(row), nil
}

func (s *Store) GetApplication(ctx context.Context, id uuid.UUID) (domain.Application, error) {
	row, err := s.q.GetApplication(ctx, pgUUID(id))
	if err != nil {
		return domain.Application{}, fmt.Errorf("get application: %w", err)
	}
	return applicationFromRow(row), nil
}

func (s *Store) ListApplications(ctx context.Context) ([]domain.Application, error) {
	rows, err := s.q.ListApplications(ctx)
	if err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}
	out := make([]domain.Application, 0, len(rows))
	for _, row := range rows {
		out = append(out, applicationFromRow(row))
	}
	return out, nil
}

// RecordStageEvent validates the transition, then writes the audit event and
// the application's denormalized current_stage together in one transaction
// so the two can never disagree.
func (s *Store) RecordStageEvent(ctx context.Context, applicationID uuid.UUID, from, to domain.Stage, detectedVia domain.DetectedVia, sourceEmailID *uuid.UUID, confidence *float64, note string) (domain.StageEvent, error) {
	if !domain.CanTransition(from, to) {
		return domain.StageEvent{}, domain.ErrInvalidTransition{From: from, To: to}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.StageEvent{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	q := s.q.WithTx(tx)

	row, err := q.InsertStageEvent(ctx, sqlc.InsertStageEventParams{
		ApplicationID: pgUUID(applicationID),
		FromStage:     string(from),
		ToStage:       string(to),
		DetectedVia:   string(detectedVia),
		SourceEmailID: pgUUIDPtr(sourceEmailID),
		Confidence:    confidence,
		Note:          note,
	})
	if err != nil {
		return domain.StageEvent{}, fmt.Errorf("insert stage event: %w", err)
	}

	if err := q.UpdateApplicationStage(ctx, sqlc.UpdateApplicationStageParams{
		ID:           pgUUID(applicationID),
		CurrentStage: string(to),
	}); err != nil {
		return domain.StageEvent{}, fmt.Errorf("update application stage: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.StageEvent{}, fmt.Errorf("commit: %w", err)
	}

	return stageEventFromRow(row), nil
}

func (s *Store) ListStageEvents(ctx context.Context, applicationID uuid.UUID) ([]domain.StageEvent, error) {
	rows, err := s.q.ListStageEventsForApplication(ctx, pgUUID(applicationID))
	if err != nil {
		return nil, fmt.Errorf("list stage events: %w", err)
	}
	out := make([]domain.StageEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, stageEventFromRow(row))
	}
	return out, nil
}

// InsertEmailMessageIfNew inserts msg, or does nothing if gmail_message_id
// already exists -- the sync worker re-lists an overlap window on every
// tick by design (see SyncWorker.tick), so duplicate IDs are the normal
// case, not an error.
func (s *Store) InsertEmailMessageIfNew(ctx context.Context, msg domain.EmailMessage) (domain.EmailMessage, bool, error) {
	row, err := s.q.InsertEmailMessageIfNew(ctx, sqlc.InsertEmailMessageIfNewParams{
		GmailMessageID: msg.GmailMessageID,
		GmailThreadID:  msg.GmailThreadID,
		FromAddress:    msg.FromAddress,
		FromDomain:     msg.FromDomain,
		Subject:        msg.Subject,
		Snippet:        msg.Snippet,
		ReceivedAt:     pgTimestamptz(msg.ReceivedAt),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.EmailMessage{}, false, nil
		}
		return domain.EmailMessage{}, false, fmt.Errorf("insert email message: %w", err)
	}
	return emailMessageFromRow(row), true, nil
}

func (s *Store) GetEmailMessageByGmailID(ctx context.Context, gmailMessageID string) (domain.EmailMessage, error) {
	row, err := s.q.GetEmailMessageByGmailID(ctx, gmailMessageID)
	if err != nil {
		return domain.EmailMessage{}, fmt.Errorf("get email message: %w", err)
	}
	return emailMessageFromRow(row), nil
}

// SetEmailClassification records the classifier's verdict for a message.
// Separate from insertion because classification (and its LLM fallback
// tier) happens after the message is already durably stored -- a
// classification failure must never lose the underlying email.
func (s *Store) SetEmailClassification(ctx context.Context, id uuid.UUID, label domain.ClassifiedLabel, confidence float64, source domain.ClassificationSource) error {
	labelStr := string(label)
	sourceStr := string(source)
	if err := s.q.SetEmailClassification(ctx, sqlc.SetEmailClassificationParams{
		ID:                       pgUUID(id),
		ClassifiedLabel:          &labelStr,
		ClassificationConfidence: &confidence,
		ClassificationSource:     &sourceStr,
	}); err != nil {
		return fmt.Errorf("set email classification: %w", err)
	}
	return nil
}

func (s *Store) GetSyncState(ctx context.Context) (domain.SyncState, error) {
	row, err := s.q.GetSyncState(ctx)
	if err != nil {
		return domain.SyncState{}, fmt.Errorf("get sync state: %w", err)
	}
	state := domain.SyncState{LastHistoryID: row.LastHistoryID}
	if row.LastSyncedAt.Valid {
		t := row.LastSyncedAt.Time
		state.LastSyncedAt = &t
	}
	return state, nil
}

func (s *Store) UpdateSyncState(ctx context.Context, state domain.SyncState) error {
	var syncedAt pgtype.Timestamptz
	if state.LastSyncedAt != nil {
		syncedAt = pgTimestamptz(*state.LastSyncedAt)
	}
	if err := s.q.UpdateSyncState(ctx, sqlc.UpdateSyncStateParams{
		LastHistoryID: state.LastHistoryID,
		LastSyncedAt:  syncedAt,
	}); err != nil {
		return fmt.Errorf("update sync state: %w", err)
	}
	return nil
}
