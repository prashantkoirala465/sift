package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/prashantkoirala465/sift/internal/classify"
	"github.com/prashantkoirala465/sift/internal/domain"
	"github.com/prashantkoirala465/sift/internal/gmail"
	"github.com/prashantkoirala465/sift/internal/match"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeGmail lets the fetchIDs/backfill/ingest decision tree be tested
// without a real Gmail client.
type fakeGmail struct {
	currentHistoryID    uint64
	currentHistoryIDErr error

	listRecentIDs []string
	listRecentErr error

	historySinceIDs    []string
	historySinceNewID  uint64
	historySinceErr    error
	historySinceCalled bool

	messages map[string]gmail.Message
}

func (f *fakeGmail) CurrentHistoryID(context.Context) (uint64, error) {
	return f.currentHistoryID, f.currentHistoryIDErr
}

func (f *fakeGmail) ListRecent(context.Context, string, int64) ([]string, error) {
	return f.listRecentIDs, f.listRecentErr
}

func (f *fakeGmail) ListHistorySince(context.Context, uint64) ([]string, uint64, error) {
	f.historySinceCalled = true
	return f.historySinceIDs, f.historySinceNewID, f.historySinceErr
}

func (f *fakeGmail) GetMessage(_ context.Context, id string) (gmail.Message, error) {
	msg, ok := f.messages[id]
	if !ok {
		return gmail.Message{}, errors.New("message not found: " + id)
	}
	return msg, nil
}

func TestFetchIDsBackfillsWhenNoHistory(t *testing.T) {
	svc := &fakeGmail{currentHistoryID: 42, listRecentIDs: []string{"a", "b"}}
	w := &SyncWorker{logger: testLogger()}

	ids, newHistoryID, err := w.fetchIDs(context.Background(), svc, domain.SyncState{LastHistoryID: ""})
	if err != nil {
		t.Fatalf("fetchIDs: %v", err)
	}
	if svc.historySinceCalled {
		t.Error("expected backfill path, but ListHistorySince was called")
	}
	if len(ids) != 2 || newHistoryID != 42 {
		t.Errorf("got ids=%v newHistoryID=%d, want [a b] 42", ids, newHistoryID)
	}
}

func TestFetchIDsIncrementalWhenHistoryPresent(t *testing.T) {
	svc := &fakeGmail{historySinceIDs: []string{"c"}, historySinceNewID: 100}
	w := &SyncWorker{logger: testLogger()}

	ids, newHistoryID, err := w.fetchIDs(context.Background(), svc, domain.SyncState{LastHistoryID: "50"})
	if err != nil {
		t.Fatalf("fetchIDs: %v", err)
	}
	if !svc.historySinceCalled {
		t.Error("expected incremental path, but ListHistorySince was not called")
	}
	if len(ids) != 1 || ids[0] != "c" || newHistoryID != 100 {
		t.Errorf("got ids=%v newHistoryID=%d, want [c] 100", ids, newHistoryID)
	}
}

func TestFetchIDsCorruptStoredHistoryForcesBackfill(t *testing.T) {
	svc := &fakeGmail{currentHistoryID: 7, listRecentIDs: []string{"z"}}
	w := &SyncWorker{logger: testLogger()}

	ids, newHistoryID, err := w.fetchIDs(context.Background(), svc, domain.SyncState{LastHistoryID: "not-a-number"})
	if err != nil {
		t.Fatalf("fetchIDs: %v", err)
	}
	if svc.historySinceCalled {
		t.Error("expected backfill on corrupt history id, but ListHistorySince was called")
	}
	if len(ids) != 1 || newHistoryID != 7 {
		t.Errorf("got ids=%v newHistoryID=%d, want [z] 7", ids, newHistoryID)
	}
}

func TestFetchIDsExpiredHistoryFallsBackToBackfill(t *testing.T) {
	svc := &fakeGmail{
		historySinceErr:  gmail.ErrHistoryExpired,
		currentHistoryID: 99,
		listRecentIDs:    []string{"fresh"},
	}
	w := &SyncWorker{logger: testLogger()}

	ids, newHistoryID, err := w.fetchIDs(context.Background(), svc, domain.SyncState{LastHistoryID: "10"})
	if err != nil {
		t.Fatalf("fetchIDs: %v", err)
	}
	if !svc.historySinceCalled {
		t.Error("expected ListHistorySince to have been attempted before falling back")
	}
	if len(ids) != 1 || ids[0] != "fresh" || newHistoryID != 99 {
		t.Errorf("got ids=%v newHistoryID=%d, want [fresh] 99 after backfill fallback", ids, newHistoryID)
	}
}

func TestFetchIDsPropagatesNonExpiredHistoryError(t *testing.T) {
	svc := &fakeGmail{historySinceErr: errors.New("gmail is down")}
	w := &SyncWorker{logger: testLogger()}

	_, _, err := w.fetchIDs(context.Background(), svc, domain.SyncState{LastHistoryID: "10"})
	if err == nil {
		t.Fatal("expected an error to propagate, got nil")
	}
}

// fakeStore is a minimal in-memory Store for exercising ingest.
type fakeStore struct {
	byGmailID map[string]domain.EmailMessage
	byID      map[uuid.UUID]domain.EmailMessage
}

func newFakeStore() *fakeStore {
	return &fakeStore{byGmailID: map[string]domain.EmailMessage{}, byID: map[uuid.UUID]domain.EmailMessage{}}
}

func (s *fakeStore) SaveToken(context.Context, *oauth2.Token) error { return nil }
func (s *fakeStore) LoadToken(context.Context) (*oauth2.Token, error) {
	return nil, gmail.ErrNoToken
}

func (s *fakeStore) GetSyncState(context.Context) (domain.SyncState, error) {
	return domain.SyncState{}, nil
}
func (s *fakeStore) UpdateSyncState(context.Context, domain.SyncState) error { return nil }

func (s *fakeStore) InsertEmailMessageIfNew(_ context.Context, msg domain.EmailMessage) (domain.EmailMessage, bool, error) {
	if _, exists := s.byGmailID[msg.GmailMessageID]; exists {
		return domain.EmailMessage{}, false, nil
	}
	msg.ID = uuid.New()
	s.byGmailID[msg.GmailMessageID] = msg
	s.byID[msg.ID] = msg
	return msg, true, nil
}

func (s *fakeStore) SetEmailClassification(_ context.Context, id uuid.UUID, label domain.ClassifiedLabel, confidence float64, source domain.ClassificationSource) error {
	msg := s.byID[id]
	msg.ClassifiedLabel = &label
	msg.ClassificationConfidence = &confidence
	msg.ClassificationSource = &source
	s.byID[id] = msg
	return nil
}

func (s *fakeStore) FindApplicationIDByThreadID(context.Context, string) (*uuid.UUID, error) {
	return nil, nil
}
func (s *fakeStore) FindApplicationIDByDomainHistory(context.Context, string) (*uuid.UUID, error) {
	return nil, nil
}
func (s *fakeStore) ListApplications(context.Context) ([]domain.Application, error) { return nil, nil }
func (s *fakeStore) SetEmailMatch(context.Context, uuid.UUID, uuid.UUID, float64, domain.ReviewStatus) error {
	return nil
}
func (s *fakeStore) GetApplication(context.Context, uuid.UUID) (domain.Application, error) {
	return domain.Application{}, errors.New("not found")
}
func (s *fakeStore) RecordStageEvent(context.Context, uuid.UUID, domain.Stage, domain.Stage, domain.DetectedVia, *uuid.UUID, *float64, string) (domain.StageEvent, error) {
	return domain.StageEvent{}, errors.New("not implemented")
}

func TestIngestSkipsFailedFetchButContinues(t *testing.T) {
	svc := &fakeGmail{
		messages: map[string]gmail.Message{
			"good": {ID: "good", ThreadID: "t1", From: "a@example.com", FromDomain: "example.com", Subject: "hi", ReceivedAt: time.Now()},
		},
	}
	store := newFakeStore()
	w := &SyncWorker{
		store:      store,
		classifier: classify.NewTieredClassifier(nil, testLogger()),
		matcher:    match.NewMatcher(store, testLogger()),
		logger:     testLogger(),
	}

	inserted := w.ingest(context.Background(), svc, []string{"missing", "good"})

	if inserted != 1 {
		t.Errorf("inserted = %d, want 1 (the failed fetch should be skipped, not fatal)", inserted)
	}
	if len(store.byGmailID) != 1 {
		t.Errorf("stored %d messages, want 1", len(store.byGmailID))
	}
}

func TestIngestSkipsAlreadySeenMessage(t *testing.T) {
	svc := &fakeGmail{
		messages: map[string]gmail.Message{
			"dup": {ID: "dup", ThreadID: "t1", From: "a@example.com", FromDomain: "example.com", Subject: "hi", ReceivedAt: time.Now()},
		},
	}
	store := newFakeStore()
	store.byGmailID["dup"] = domain.EmailMessage{GmailMessageID: "dup"} // pre-seed as already ingested

	w := &SyncWorker{
		store:      store,
		classifier: classify.NewTieredClassifier(nil, testLogger()),
		matcher:    match.NewMatcher(store, testLogger()),
		logger:     testLogger(),
	}

	inserted := w.ingest(context.Background(), svc, []string{"dup"})
	if inserted != 0 {
		t.Errorf("inserted = %d, want 0 for an already-seen message", inserted)
	}
}
