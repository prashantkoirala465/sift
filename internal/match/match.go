// Package match links an incoming, already-classified email to the
// Application it's about. A confident match also drives the pipeline
// forward: if the email's classification implies a stage the state
// machine allows from the application's current stage, Resolve records
// that transition itself.
package match

import (
	"context"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/prashantkoirala465/sift/internal/classify"
	"github.com/prashantkoirala465/sift/internal/domain"
)

// autoApplyThreshold: a match below this confidence is still recorded (so
// a review queue can show "we think it's this one"), but doesn't get
// review_status=matched and never drives a stage transition.
const autoApplyThreshold = 0.7

// Per-method confidence, highest signal first. See find's doc comment for
// why each tier is ordered the way it is.
const (
	confidenceThread        = 0.99
	confidenceDomainHistory = 0.85
	confidenceFuzzyName     = 0.6
)

// labelStageTransitions is the set of classification labels that imply a
// pipeline stage change, and which stage they imply. Assessment,
// confirmation, other, and unclassified mail don't move the pipeline on
// their own -- there's no stage in the model for "assessment requested",
// and a confirmation email doesn't advance anything.
var labelStageTransitions = map[domain.ClassifiedLabel]domain.Stage{
	domain.LabelRejection: domain.StageRejected,
	domain.LabelOffer:     domain.StageOffer,
	domain.LabelInterview: domain.StageInterview,
}

// Store is what the matcher needs from storage, kept narrow so this
// package doesn't depend on Postgres directly.
type Store interface {
	FindApplicationIDByThreadID(ctx context.Context, threadID string) (*uuid.UUID, error)
	FindApplicationIDByDomainHistory(ctx context.Context, fromDomain string) (*uuid.UUID, error)
	ListApplications(ctx context.Context) ([]domain.Application, error)
	SetEmailMatch(ctx context.Context, emailID, applicationID uuid.UUID, confidence float64, status domain.ReviewStatus) error
	GetApplication(ctx context.Context, id uuid.UUID) (domain.Application, error)
	RecordStageEvent(ctx context.Context, applicationID uuid.UUID, from, to domain.Stage, detectedVia domain.DetectedVia, sourceEmailID *uuid.UUID, confidence *float64, note string) (domain.StageEvent, error)
}

type Matcher struct {
	store  Store
	logger *slog.Logger
}

func NewMatcher(store Store, logger *slog.Logger) *Matcher {
	return &Matcher{store: store, logger: logger}
}

type candidate struct {
	applicationID uuid.UUID
	confidence    float64
}

// Resolve finds the application msg belongs to, if any, records the match,
// and -- only when confidence clears autoApplyThreshold and the
// classified label implies a state-machine-valid transition -- records
// the stage event. A confident match whose implied transition isn't valid
// from the application's current stage still gets linked; it just doesn't
// auto-advance, since forcing a skipped stage would defeat the point of
// having a state machine at all. That gap is for a human to close, not
// this package to paper over.
func (m *Matcher) Resolve(ctx context.Context, msg domain.EmailMessage) error {
	cand, err := m.find(ctx, msg)
	if err != nil {
		return err
	}
	if cand == nil {
		return nil // stays review_status=pending, unmatched
	}

	status := domain.ReviewStatusPending
	if cand.confidence >= autoApplyThreshold {
		status = domain.ReviewStatusMatched
	}
	if err := m.store.SetEmailMatch(ctx, msg.ID, cand.applicationID, cand.confidence, status); err != nil {
		return err
	}
	if status != domain.ReviewStatusMatched {
		return nil
	}

	return m.maybeAdvance(ctx, msg, cand)
}

func (m *Matcher) maybeAdvance(ctx context.Context, msg domain.EmailMessage, cand *candidate) error {
	if msg.ClassifiedLabel == nil {
		return nil
	}
	targetStage, ok := labelStageTransitions[*msg.ClassifiedLabel]
	if !ok {
		return nil
	}

	app, err := m.store.GetApplication(ctx, cand.applicationID)
	if err != nil {
		return err
	}
	if !domain.CanTransition(app.CurrentStage, targetStage) {
		m.logger.Info("matched email implies a transition the state machine won't allow, leaving for manual review",
			"application_id", cand.applicationID, "from", app.CurrentStage, "to", targetStage)
		return nil
	}

	confidence := cand.confidence
	_, err = m.store.RecordStageEvent(ctx, cand.applicationID, app.CurrentStage, targetStage,
		domain.DetectedViaEmailAuto, &msg.ID, &confidence, "")
	return err
}

// find tries each signal in order of reliability, returning the first
// hit: thread continuity (an earlier email in the same thread was already
// matched) beats domain history (every past match from this sender domain
// agrees) beats a literal company-name mention in the subject/snippet.
// Domain history is skipped for shared ATS domains -- greenhouse.io tells
// you nothing about which company an email is from.
func (m *Matcher) find(ctx context.Context, msg domain.EmailMessage) (*candidate, error) {
	if id, err := m.store.FindApplicationIDByThreadID(ctx, msg.GmailThreadID); err != nil {
		return nil, err
	} else if id != nil {
		return &candidate{applicationID: *id, confidence: confidenceThread}, nil
	}

	if !classify.IsKnownATSDomain(msg.FromDomain) {
		if id, err := m.store.FindApplicationIDByDomainHistory(ctx, msg.FromDomain); err != nil {
			return nil, err
		} else if id != nil {
			return &candidate{applicationID: *id, confidence: confidenceDomainHistory}, nil
		}
	}

	return m.findByCompanyName(ctx, msg)
}

func (m *Matcher) findByCompanyName(ctx context.Context, msg domain.EmailMessage) (*candidate, error) {
	apps, err := m.store.ListApplications(ctx)
	if err != nil {
		return nil, err
	}

	text := strings.ToLower(msg.Subject + " " + msg.Snippet)
	for _, app := range apps {
		name := normalizeCompanyName(app.Company)
		if name != "" && strings.Contains(text, name) {
			return &candidate{applicationID: app.ID, confidence: confidenceFuzzyName}, nil
		}
	}
	return nil, nil
}
