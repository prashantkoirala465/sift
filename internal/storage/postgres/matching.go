package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/prashantkoirala465/sift/internal/domain"
	"github.com/prashantkoirala465/sift/internal/storage/postgres/sqlc"
)

// FindApplicationIDByThreadID returns the application already linked to
// this Gmail thread, if any -- the strongest matching signal there is.
func (s *Store) FindApplicationIDByThreadID(ctx context.Context, threadID string) (*uuid.UUID, error) {
	row, err := s.q.FindApplicationIDByThreadID(ctx, threadID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find application by thread id: %w", err)
	}
	id := fromPgUUID(row)
	return &id, nil
}

// FindApplicationIDByDomainHistory returns the application this sender
// domain has always resolved to in the past, or nil if the domain has no
// history or -- e.g. a shared ATS domain that happens to have been linked
// to more than one application -- resolves ambiguously.
func (s *Store) FindApplicationIDByDomainHistory(ctx context.Context, fromDomain string) (*uuid.UUID, error) {
	rows, err := s.q.ListDistinctMatchedApplicationsByDomain(ctx, fromDomain)
	if err != nil {
		return nil, fmt.Errorf("list applications by domain history: %w", err)
	}
	if len(rows) != 1 {
		return nil, nil
	}
	id := fromPgUUID(rows[0])
	return &id, nil
}

// SetEmailMatch records the best matching guess for an email, whether or
// not it clears the auto-apply confidence threshold -- an unconfirmed
// guess is still worth showing in a review queue.
func (s *Store) SetEmailMatch(ctx context.Context, emailID, applicationID uuid.UUID, confidence float64, status domain.ReviewStatus) error {
	if err := s.q.SetEmailMatch(ctx, sqlc.SetEmailMatchParams{
		ID:                   pgUUID(emailID),
		MatchedApplicationID: pgUUID(applicationID),
		MatchConfidence:      &confidence,
		ReviewStatus:         string(status),
	}); err != nil {
		return fmt.Errorf("set email match: %w", err)
	}
	return nil
}
