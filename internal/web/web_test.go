package web_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/prashantkoirala465/sift/internal/domain"
	"github.com/prashantkoirala465/sift/internal/security"
	"github.com/prashantkoirala465/sift/internal/storage/postgres"
	"github.com/prashantkoirala465/sift/internal/web"
)

func requireMux(t *testing.T) (*http.ServeMux, *postgres.Store) {
	t.Helper()

	dsn := os.Getenv("SIFT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SIFT_TEST_DATABASE_URL not set, skipping web integration test")
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
	mux := http.NewServeMux()
	web.RegisterRoutes(mux, web.Deps{Store: store})
	return mux, store
}

func postForm(t *testing.T, mux http.Handler, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestDashboardRenders(t *testing.T) {
	mux, _ := requireMux(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Applications") {
		t.Errorf("dashboard body missing expected heading, got: %s", rec.Body.String())
	}
}

func TestCreateApplicationFormRedirectsAndAppearsOnBoard(t *testing.T) {
	mux, _ := requireMux(t)

	rec := postForm(t, mux, "/applications", url.Values{
		"company":      {"Web Form Co"},
		"role_title":   {"Engineer"},
		"source":       {"referral"},
		"applied_date": {time.Now().Format("2006-01-02")},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusSeeOther, rec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	dashRec := httptest.NewRecorder()
	mux.ServeHTTP(dashRec, req)
	if !strings.Contains(dashRec.Body.String(), "Web Form Co") {
		t.Errorf("dashboard does not show the newly created application")
	}
}

func TestReviewQueueConfirmViaHTMXFormAppliesTransition(t *testing.T) {
	mux, store := requireMux(t)
	ctx := context.Background()

	app, err := store.CreateApplication(ctx, "Web Review Co", "SRE", domain.SourceJobBoard, time.Now())
	if err != nil {
		t.Fatalf("create application: %v", err)
	}
	if _, err := store.RecordStageEvent(ctx, app.ID, domain.StageApplied, domain.StageScreening, domain.DetectedViaManual, nil, nil, ""); err != nil {
		t.Fatalf("advance to screening: %v", err)
	}

	msg, _, err := store.InsertEmailMessageIfNew(ctx, domain.EmailMessage{
		GmailMessageID: "web-review-test-" + app.ID.String(),
		GmailThreadID:  "thread-web-review",
		FromAddress:    "recruiter@example.com",
		FromDomain:     "example.com",
		Subject:        "Interview scheduling",
		ReceivedAt:     time.Now(),
	})
	if err != nil {
		t.Fatalf("insert email message: %v", err)
	}
	if err := store.SetEmailClassification(ctx, msg.ID, domain.LabelInterview, 0.8, domain.ClassificationSourceRule); err != nil {
		t.Fatalf("set classification: %v", err)
	}

	rec := postForm(t, mux, "/review-queue/"+msg.ID.String()+"/confirm", url.Values{
		"application_id": {app.ID.String()},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm status = %d, body = %s", rec.Code, rec.Body.String())
	}

	updated, err := store.GetApplication(ctx, app.ID)
	if err != nil {
		t.Fatalf("get application: %v", err)
	}
	if updated.CurrentStage != domain.StageInterview {
		t.Errorf("CurrentStage = %s, want %s", updated.CurrentStage, domain.StageInterview)
	}
}
