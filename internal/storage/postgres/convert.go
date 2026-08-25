package postgres

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/prashantkoirala465/sift/internal/domain"
	"github.com/prashantkoirala465/sift/internal/storage/postgres/sqlc"
)

func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func fromPgUUID(id pgtype.UUID) uuid.UUID {
	return uuid.UUID(id.Bytes)
}

func fromPgUUIDPtr(id pgtype.UUID) *uuid.UUID {
	if !id.Valid {
		return nil
	}
	u := uuid.UUID(id.Bytes)
	return &u
}

func pgUUIDPtr(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgUUID(*id)
}

func pgDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}

func pgTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func applicationFromRow(row sqlc.Application) domain.Application {
	return domain.Application{
		ID:           fromPgUUID(row.ID),
		Company:      row.Company,
		RoleTitle:    row.RoleTitle,
		Source:       domain.Source(row.Source),
		AppliedDate:  row.AppliedDate.Time,
		CurrentStage: domain.Stage(row.CurrentStage),
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}
}

func stageEventFromRow(row sqlc.StageEvent) domain.StageEvent {
	return domain.StageEvent{
		ID:            fromPgUUID(row.ID),
		ApplicationID: fromPgUUID(row.ApplicationID),
		FromStage:     domain.Stage(row.FromStage),
		ToStage:       domain.Stage(row.ToStage),
		DetectedVia:   domain.DetectedVia(row.DetectedVia),
		SourceEmailID: fromPgUUIDPtr(row.SourceEmailID),
		Confidence:    row.Confidence,
		Note:          row.Note,
		OccurredAt:    row.OccurredAt.Time,
	}
}

func emailMessageFromRow(row sqlc.EmailMessage) domain.EmailMessage {
	msg := domain.EmailMessage{
		ID:                       fromPgUUID(row.ID),
		GmailMessageID:           row.GmailMessageID,
		GmailThreadID:            row.GmailThreadID,
		FromAddress:              row.FromAddress,
		FromDomain:               row.FromDomain,
		Subject:                  row.Subject,
		ReceivedAt:               row.ReceivedAt.Time,
		ClassificationConfidence: row.ClassificationConfidence,
		MatchedApplicationID:     fromPgUUIDPtr(row.MatchedApplicationID),
		MatchConfidence:          row.MatchConfidence,
		ReviewStatus:             domain.ReviewStatus(row.ReviewStatus),
	}

	if row.ClassifiedLabel != nil {
		label := domain.ClassifiedLabel(*row.ClassifiedLabel)
		msg.ClassifiedLabel = &label
	}
	if row.ClassificationSource != nil {
		src := domain.ClassificationSource(*row.ClassificationSource)
		msg.ClassificationSource = &src
	}
	if row.ProcessedAt.Valid {
		t := row.ProcessedAt.Time
		msg.ProcessedAt = &t
	}
	if row.CreatedAt.Valid {
		msg.CreatedAt = row.CreatedAt.Time
	}

	return msg
}
