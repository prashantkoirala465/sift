package web

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/prashantkoirala465/sift/internal/domain"
)

// Store is everything the web handlers need from storage, kept narrow so
// this package doesn't depend on Postgres directly. Shape mirrors
// api.Store deliberately -- they're independent presentation layers over
// the same domain, declared separately rather than sharing a type so
// neither depends on the other.
type Store interface {
	CreateApplication(ctx context.Context, company, roleTitle string, source domain.Source, appliedDate time.Time) (domain.Application, error)
	GetApplication(ctx context.Context, id uuid.UUID) (domain.Application, error)
	ListApplications(ctx context.Context) ([]domain.Application, error)
	RecordStageEvent(ctx context.Context, applicationID uuid.UUID, from, to domain.Stage, detectedVia domain.DetectedVia, sourceEmailID *uuid.UUID, confidence *float64, note string) (domain.StageEvent, error)
	ListStageEvents(ctx context.Context, applicationID uuid.UUID) ([]domain.StageEvent, error)

	GetEmailMessage(ctx context.Context, id uuid.UUID) (domain.EmailMessage, error)
	ListEmailMessagesByReviewStatus(ctx context.Context, status domain.ReviewStatus) ([]domain.EmailMessage, error)
	SetEmailMatch(ctx context.Context, emailID, applicationID uuid.UUID, confidence float64, status domain.ReviewStatus) error
	SetEmailReviewStatus(ctx context.Context, id uuid.UUID, status domain.ReviewStatus) error
}
