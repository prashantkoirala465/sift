package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/prashantkoirala465/sift/internal/api"
	"github.com/prashantkoirala465/sift/internal/domain"
	"github.com/prashantkoirala465/sift/internal/security"
	"github.com/prashantkoirala465/sift/internal/storage/postgres"
)

func requireRouter(t *testing.T) (http.Handler, *postgres.Store) {
	t.Helper()

	dsn := os.Getenv("SIFT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SIFT_TEST_DATABASE_URL not set, skipping API integration test")
	}
	if err := postgres.Migrate(dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := postgres.NewPool(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	store := postgres.NewStore(pool, make([]byte, security.KeySize))
	router := api.NewRouter(api.Deps{Store: store})
	return router, store
}

func doJSON(t *testing.T, router http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestApplicationLifecycle(t *testing.T) {
	router, _ := requireRouter(t)

	createRec := doJSON(t, router, http.MethodPost, "/api/applications", map[string]string{
		"company":      "Acme Corp",
		"role_title":   "Backend Engineer",
		"source":       "referral",
		"applied_date": time.Now().Format("2006-01-02"),
	})
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		ID           string `json:"id"`
		CurrentStage string `json:"current_stage"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.CurrentStage != string(domain.StageApplied) {
		t.Errorf("current_stage = %s, want %s", created.CurrentStage, domain.StageApplied)
	}

	getRec := doJSON(t, router, http.MethodGet, "/api/applications/"+created.ID, nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", getRec.Code, getRec.Body.String())
	}

	validRec := doJSON(t, router, http.MethodPost, "/api/applications/"+created.ID+"/stage", map[string]string{
		"to_stage": "screening",
	})
	if validRec.Code != http.StatusOK {
		t.Fatalf("valid stage transition status = %d, body = %s", validRec.Code, validRec.Body.String())
	}

	invalidRec := doJSON(t, router, http.MethodPost, "/api/applications/"+created.ID+"/stage", map[string]string{
		"to_stage": "offer", // screening -> offer skips interview
	})
	if invalidRec.Code != http.StatusConflict {
		t.Fatalf("invalid stage transition status = %d, want %d, body = %s", invalidRec.Code, http.StatusConflict, invalidRec.Body.String())
	}
}

func TestReviewQueueConfirmAppliesImpliedTransition(t *testing.T) {
	router, store := requireRouter(t)
	ctx := context.Background()

	app, err := store.CreateApplication(ctx, "Globex", "Platform Engineer", domain.SourceLinkedIn, time.Now())
	if err != nil {
		t.Fatalf("create application: %v", err)
	}
	if _, err := store.RecordStageEvent(ctx, app.ID, domain.StageApplied, domain.StageScreening, domain.DetectedViaManual, nil, nil, ""); err != nil {
		t.Fatalf("advance to screening: %v", err)
	}

	msg, _, err := store.InsertEmailMessageIfNew(ctx, domain.EmailMessage{
		GmailMessageID: "review-queue-test-" + app.ID.String(),
		GmailThreadID:  "thread-1",
		FromAddress:    "recruiter@globex.example",
		FromDomain:     "globex.example",
		Subject:        "Let's schedule an interview",
		ReceivedAt:     time.Now(),
	})
	if err != nil {
		t.Fatalf("insert email message: %v", err)
	}
	if err := store.SetEmailClassification(ctx, msg.ID, domain.LabelInterview, 0.9, domain.ClassificationSourceRule); err != nil {
		t.Fatalf("set classification: %v", err)
	}

	listRec := doJSON(t, router, http.MethodGet, "/api/review-queue", nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list review queue status = %d, body = %s", listRec.Code, listRec.Body.String())
	}

	confirmRec := doJSON(t, router, http.MethodPost, "/api/review-queue/"+msg.ID.String()+"/confirm", map[string]string{
		"application_id": app.ID.String(),
	})
	if confirmRec.Code != http.StatusNoContent {
		t.Fatalf("confirm status = %d, body = %s", confirmRec.Code, confirmRec.Body.String())
	}

	updated, err := store.GetApplication(ctx, app.ID)
	if err != nil {
		t.Fatalf("get application: %v", err)
	}
	if updated.CurrentStage != domain.StageInterview {
		t.Errorf("application.CurrentStage = %s, want %s after confirming an interview email", updated.CurrentStage, domain.StageInterview)
	}
}

func TestReviewQueueIgnore(t *testing.T) {
	router, store := requireRouter(t)
	ctx := context.Background()

	msg, _, err := store.InsertEmailMessageIfNew(ctx, domain.EmailMessage{
		GmailMessageID: "review-queue-ignore-test-" + uuid.NewString(),
		GmailThreadID:  "thread-2",
		FromAddress:    "newsletter@example.com",
		FromDomain:     "example.com",
		Subject:        "Weekly digest",
		ReceivedAt:     time.Now(),
	})
	if err != nil {
		t.Fatalf("insert email message: %v", err)
	}

	rec := doJSON(t, router, http.MethodPost, "/api/review-queue/"+msg.ID.String()+"/ignore", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("ignore status = %d, body = %s", rec.Code, rec.Body.String())
	}

	got, err := store.GetEmailMessage(ctx, msg.ID)
	if err != nil {
		t.Fatalf("get email message: %v", err)
	}
	if got.ReviewStatus != domain.ReviewStatusIgnored {
		t.Errorf("ReviewStatus = %s, want %s", got.ReviewStatus, domain.ReviewStatusIgnored)
	}
}
