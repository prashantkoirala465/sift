package match

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"

	"github.com/prashantkoirala465/sift/internal/domain"
)

type matchCall struct {
	emailID       uuid.UUID
	applicationID uuid.UUID
	confidence    float64
	status        domain.ReviewStatus
}

type stageCall struct {
	applicationID uuid.UUID
	from, to      domain.Stage
}

type fakeStore struct {
	threadMatch   map[string]uuid.UUID
	domainMatches map[string][]uuid.UUID
	applications  []domain.Application
	appsByID      map[uuid.UUID]domain.Application

	matchCalls []matchCall
	stageCalls []stageCall
}

func (f *fakeStore) FindApplicationIDByThreadID(_ context.Context, threadID string) (*uuid.UUID, error) {
	if id, ok := f.threadMatch[threadID]; ok {
		return &id, nil
	}
	return nil, nil
}

func (f *fakeStore) FindApplicationIDByDomainHistory(_ context.Context, fromDomain string) (*uuid.UUID, error) {
	ids := f.domainMatches[fromDomain]
	if len(ids) != 1 {
		return nil, nil
	}
	return &ids[0], nil
}

func (f *fakeStore) ListApplications(_ context.Context) ([]domain.Application, error) {
	return f.applications, nil
}

func (f *fakeStore) SetEmailMatch(_ context.Context, emailID, applicationID uuid.UUID, confidence float64, status domain.ReviewStatus) error {
	f.matchCalls = append(f.matchCalls, matchCall{emailID, applicationID, confidence, status})
	return nil
}

func (f *fakeStore) GetApplication(_ context.Context, id uuid.UUID) (domain.Application, error) {
	return f.appsByID[id], nil
}

func (f *fakeStore) RecordStageEvent(_ context.Context, applicationID uuid.UUID, from, to domain.Stage, _ domain.DetectedVia, _ *uuid.UUID, _ *float64, _ string) (domain.StageEvent, error) {
	f.stageCalls = append(f.stageCalls, stageCall{applicationID, from, to})
	return domain.StageEvent{ApplicationID: applicationID, FromStage: from, ToStage: to}, nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestResolveThreadContinuityAppliesValidTransition(t *testing.T) {
	appID := uuid.New()
	emailID := uuid.New()
	label := domain.LabelInterview

	store := &fakeStore{
		threadMatch: map[string]uuid.UUID{"t1": appID},
		appsByID:    map[uuid.UUID]domain.Application{appID: {ID: appID, CurrentStage: domain.StageScreening}},
	}
	m := NewMatcher(store, testLogger())

	err := m.Resolve(context.Background(), domain.EmailMessage{
		ID:              emailID,
		GmailThreadID:   "t1",
		ClassifiedLabel: &label,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(store.matchCalls) != 1 {
		t.Fatalf("got %d SetEmailMatch calls, want 1", len(store.matchCalls))
	}
	got := store.matchCalls[0]
	if got.applicationID != appID || got.confidence != confidenceThread || got.status != domain.ReviewStatusMatched {
		t.Errorf("match call = %+v, want thread match on %s at confidence %v, status matched", got, appID, confidenceThread)
	}

	if len(store.stageCalls) != 1 {
		t.Fatalf("got %d RecordStageEvent calls, want 1", len(store.stageCalls))
	}
	if stage := store.stageCalls[0]; stage.from != domain.StageScreening || stage.to != domain.StageInterview {
		t.Errorf("stage transition = %s -> %s, want screening -> interview", stage.from, stage.to)
	}
}

func TestResolveConfidentMatchButInvalidTransitionSkipsStageEvent(t *testing.T) {
	appID := uuid.New()
	label := domain.LabelOffer // offer directly from "applied" is not a valid transition

	store := &fakeStore{
		threadMatch: map[string]uuid.UUID{"t2": appID},
		appsByID:    map[uuid.UUID]domain.Application{appID: {ID: appID, CurrentStage: domain.StageApplied}},
	}
	m := NewMatcher(store, testLogger())

	err := m.Resolve(context.Background(), domain.EmailMessage{
		ID:              uuid.New(),
		GmailThreadID:   "t2",
		ClassifiedLabel: &label,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(store.matchCalls) != 1 || store.matchCalls[0].status != domain.ReviewStatusMatched {
		t.Fatalf("expected a confident match to still be recorded, got %+v", store.matchCalls)
	}
	if len(store.stageCalls) != 0 {
		t.Errorf("expected no stage event for an invalid transition, got %+v", store.stageCalls)
	}
}

func TestResolveFuzzyNameMatchBelowThresholdStaysPending(t *testing.T) {
	appID := uuid.New()
	label := domain.LabelRejection

	store := &fakeStore{
		applications: []domain.Application{
			{ID: appID, Company: "Acme Corp", CurrentStage: domain.StageApplied},
		},
	}
	m := NewMatcher(store, testLogger())

	err := m.Resolve(context.Background(), domain.EmailMessage{
		ID:              uuid.New(),
		Subject:         "An update from Acme",
		FromDomain:      "random-recruiter.example",
		ClassifiedLabel: &label,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(store.matchCalls) != 1 {
		t.Fatalf("got %d SetEmailMatch calls, want 1", len(store.matchCalls))
	}
	got := store.matchCalls[0]
	if got.status != domain.ReviewStatusPending {
		t.Errorf("status = %s, want pending (fuzzy match confidence is below auto-apply threshold)", got.status)
	}
	if len(store.stageCalls) != 0 {
		t.Errorf("expected no stage event for a below-threshold match, got %+v", store.stageCalls)
	}
}

func TestResolveSkipsDomainHistoryForSharedATSDomain(t *testing.T) {
	otherAppID := uuid.New()

	store := &fakeStore{
		domainMatches: map[string][]uuid.UUID{"greenhouse.io": {otherAppID}},
	}
	m := NewMatcher(store, testLogger())

	err := m.Resolve(context.Background(), domain.EmailMessage{
		ID:         uuid.New(),
		FromDomain: "greenhouse.io",
		Subject:    "no company name mentioned here",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(store.matchCalls) != 0 {
		t.Errorf("expected no match via a shared ATS domain's history, got %+v", store.matchCalls)
	}
}

func TestResolveNoSignalNoMatch(t *testing.T) {
	store := &fakeStore{}
	m := NewMatcher(store, testLogger())

	err := m.Resolve(context.Background(), domain.EmailMessage{
		ID:         uuid.New(),
		FromDomain: "totally-unrelated.example",
		Subject:    "hello",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(store.matchCalls) != 0 {
		t.Errorf("expected no match, got %+v", store.matchCalls)
	}
}
