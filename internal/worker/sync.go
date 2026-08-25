// Package worker runs Sift's background jobs: syncing Gmail, classifying
// each newly-stored message, and matching it to a tracked application.
package worker

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"golang.org/x/oauth2"

	"github.com/google/uuid"

	"github.com/prashantkoirala465/sift/internal/classify"
	"github.com/prashantkoirala465/sift/internal/domain"
	"github.com/prashantkoirala465/sift/internal/gmail"
	"github.com/prashantkoirala465/sift/internal/match"
	"github.com/prashantkoirala465/sift/internal/observability"
)

// Store is what the sync worker needs from storage, kept narrow so this
// package doesn't depend on Postgres directly.
type Store interface {
	gmail.TokenStore
	match.Store
	GetSyncState(ctx context.Context) (domain.SyncState, error)
	UpdateSyncState(ctx context.Context, state domain.SyncState) error
	InsertEmailMessageIfNew(ctx context.Context, msg domain.EmailMessage) (domain.EmailMessage, bool, error)
	SetEmailClassification(ctx context.Context, id uuid.UUID, label domain.ClassifiedLabel, confidence float64, source domain.ClassificationSource) error
}

// GmailService is the subset of *gmail.Service the sync worker calls.
// Narrowed to an interface purely so the worker's decision logic
// (backfill vs incremental, history-expired fallback) can be tested
// against a fake instead of a real Gmail client.
type GmailService interface {
	CurrentHistoryID(ctx context.Context) (uint64, error)
	ListRecent(ctx context.Context, query string, maxResults int64) ([]string, error)
	ListHistorySince(ctx context.Context, historyID uint64) ([]string, uint64, error)
	GetMessage(ctx context.Context, id string) (gmail.Message, error)
}

// initialBackfillQuery bounds the very first sync (and any resync forced by
// an expired history checkpoint) to recent mail -- scanning someone's
// entire mailbox history would be slow and mostly irrelevant to active
// applications.
const (
	initialBackfillQuery   = "newer_than:90d"
	initialBackfillMaxMsgs = 500
)

type SyncWorker struct {
	store      Store
	oauth      *oauth2.Config
	classifier *classify.TieredClassifier
	matcher    *match.Matcher
	logger     *slog.Logger
}

func NewSyncWorker(store Store, oauthCfg *oauth2.Config, classifier *classify.TieredClassifier, matcher *match.Matcher, logger *slog.Logger) *SyncWorker {
	return &SyncWorker{store: store, oauth: oauthCfg, classifier: classifier, matcher: matcher, logger: logger}
}

// Run ticks every interval until ctx is cancelled. A failed tick is logged
// and retried next tick -- never fatal to the process. A self-hosted tool
// syncing someone's email shouldn't crash-loop because Gmail returned one
// bad response.
func (w *SyncWorker) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *SyncWorker) tick(ctx context.Context) {
	if w.oauth == nil {
		return // Google OAuth not configured at all
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	observability.SyncTicksTotal.Add(1)

	httpClient, err := gmail.HTTPClient(ctx, w.oauth, w.store)
	if err != nil {
		if errors.Is(err, gmail.ErrNoToken) {
			w.logger.Debug("sync tick skipped: gmail not connected")
			return
		}
		observability.SyncTickErrorsTotal.Add(1)
		w.logger.Error("sync tick: build gmail client", "error", err)
		return
	}

	svc, err := gmail.NewService(ctx, httpClient)
	if err != nil {
		observability.SyncTickErrorsTotal.Add(1)
		w.logger.Error("sync tick: create gmail service", "error", err)
		return
	}

	state, err := w.store.GetSyncState(ctx)
	if err != nil {
		observability.SyncTickErrorsTotal.Add(1)
		w.logger.Error("sync tick: load sync state", "error", err)
		return
	}

	ids, newHistoryID, err := w.fetchIDs(ctx, svc, state)
	if err != nil {
		observability.SyncTickErrorsTotal.Add(1)
		w.logger.Error("sync tick: fetch message ids", "error", err)
		return
	}
	observability.EmailsSeenTotal.Add(int64(len(ids)))

	inserted := w.ingest(ctx, svc, ids)

	now := time.Now().UTC()
	if err := w.store.UpdateSyncState(ctx, domain.SyncState{
		LastHistoryID: strconv.FormatUint(newHistoryID, 10),
		LastSyncedAt:  &now,
	}); err != nil {
		observability.SyncTickErrorsTotal.Add(1)
		w.logger.Error("sync tick: save sync state", "error", err)
		return
	}

	if len(ids) > 0 {
		w.logger.Info("sync tick complete", "seen", len(ids), "inserted", inserted)
	}
}

// fetchIDs decides between an incremental History.list and a bounded
// backfill, and captures the new checkpoint before processing any message
// so a message that arrives mid-tick is picked up next tick rather than
// dropped.
func (w *SyncWorker) fetchIDs(ctx context.Context, svc GmailService, state domain.SyncState) ([]string, uint64, error) {
	if state.LastHistoryID == "" {
		return w.backfill(ctx, svc)
	}

	startID, err := strconv.ParseUint(state.LastHistoryID, 10, 64)
	if err != nil {
		w.logger.Error("sync tick: stored history id is corrupt, forcing backfill", "error", err, "value", state.LastHistoryID)
		return w.backfill(ctx, svc)
	}

	ids, newHistoryID, err := svc.ListHistorySince(ctx, startID)
	if errors.Is(err, gmail.ErrHistoryExpired) {
		w.logger.Warn("gmail history checkpoint expired, falling back to backfill")
		return w.backfill(ctx, svc)
	}
	return ids, newHistoryID, err
}

func (w *SyncWorker) backfill(ctx context.Context, svc GmailService) ([]string, uint64, error) {
	checkpoint, err := svc.CurrentHistoryID(ctx)
	if err != nil {
		return nil, 0, err
	}
	ids, err := svc.ListRecent(ctx, initialBackfillQuery, initialBackfillMaxMsgs)
	if err != nil {
		return nil, 0, err
	}
	w.logger.Info("gmail backfill", "candidate_messages", len(ids))
	return ids, checkpoint, nil
}

// ingest fetches and stores each message. One bad message (deleted between
// the history event and the fetch, malformed headers) is logged and
// skipped rather than aborting the whole tick.
func (w *SyncWorker) ingest(ctx context.Context, svc GmailService, ids []string) int {
	inserted := 0
	for _, id := range ids {
		msg, err := svc.GetMessage(ctx, id)
		if err != nil {
			w.logger.Warn("sync tick: fetch message failed, skipping", "gmail_message_id", id, "error", err)
			continue
		}

		stored, isNew, err := w.store.InsertEmailMessageIfNew(ctx, domain.EmailMessage{
			GmailMessageID: msg.ID,
			GmailThreadID:  msg.ThreadID,
			FromAddress:    msg.From,
			FromDomain:     msg.FromDomain,
			Subject:        msg.Subject,
			Snippet:        msg.Snippet,
			ReceivedAt:     msg.ReceivedAt,
		})
		if err != nil {
			w.logger.Error("sync tick: store message failed, skipping", "gmail_message_id", id, "error", err)
			continue
		}
		if !isNew {
			continue
		}
		inserted++
		observability.EmailsIngestedTotal.Add(1)

		// Classification failure here would only lose the label, never the
		// message -- it's already durably stored above.
		result := w.classifier.Classify(ctx, classify.Input{
			Subject:    msg.Subject,
			Snippet:    msg.Snippet,
			FromDomain: msg.FromDomain,
		})
		observability.ClassificationsBySource.Add(string(result.Source), 1)
		if err := w.store.SetEmailClassification(ctx, stored.ID, result.Label, result.Confidence, result.Source); err != nil {
			w.logger.Error("sync tick: save classification failed", "gmail_message_id", id, "error", err)
			continue
		}

		stored.ClassifiedLabel = &result.Label
		if err := w.matcher.Resolve(ctx, stored); err != nil {
			w.logger.Error("sync tick: match/resolve failed", "gmail_message_id", id, "error", err)
		}
	}
	return inserted
}
