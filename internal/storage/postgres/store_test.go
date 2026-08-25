package postgres_test

import (
	"context"
	"os"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/prashantkoirala465/sift/internal/domain"
	"github.com/prashantkoirala465/sift/internal/security"
	"github.com/prashantkoirala465/sift/internal/storage/postgres"
)

var testEncryptionKey = make([]byte, security.KeySize) // all-zero key: fine for tests, never for real use

// requireStore skips the test unless SIFT_TEST_DATABASE_URL points at a real
// Postgres instance. Migrations run against it before every test to keep
// each test isolated from schema drift, not from other tests' data --
// tests still share the same database and must clean up after themselves.
func requireStore(t *testing.T) *postgres.Store {
	t.Helper()

	dsn := os.Getenv("SIFT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SIFT_TEST_DATABASE_URL not set, skipping Postgres integration test")
	}

	if err := postgres.Migrate(dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	pool, err := postgres.NewPool(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	return postgres.NewStore(pool, testEncryptionKey)
}

func TestApplicationRoundTrip(t *testing.T) {
	store := requireStore(t)
	ctx := context.Background()

	created, err := store.CreateApplication(ctx, "Acme Corp", "Backend Engineer", domain.SourceReferral, time.Now().UTC().Truncate(24*time.Hour))
	if err != nil {
		t.Fatalf("create application: %v", err)
	}
	if created.CurrentStage != domain.StageApplied {
		t.Errorf("new application stage = %s, want %s", created.CurrentStage, domain.StageApplied)
	}

	got, err := store.GetApplication(ctx, created.ID)
	if err != nil {
		t.Fatalf("get application: %v", err)
	}
	if got.Company != "Acme Corp" || got.RoleTitle != "Backend Engineer" {
		t.Errorf("got application %+v, want company/role to round-trip", got)
	}
}

func TestRecordStageEventValidTransition(t *testing.T) {
	store := requireStore(t)
	ctx := context.Background()

	app, err := store.CreateApplication(ctx, "Globex", "Platform Engineer", domain.SourceLinkedIn, time.Now().UTC())
	if err != nil {
		t.Fatalf("create application: %v", err)
	}

	event, err := store.RecordStageEvent(ctx, app.ID, domain.StageApplied, domain.StageScreening, domain.DetectedViaManual, nil, nil, "recruiter reached out")
	if err != nil {
		t.Fatalf("record stage event: %v", err)
	}
	if event.ToStage != domain.StageScreening {
		t.Errorf("event.ToStage = %s, want %s", event.ToStage, domain.StageScreening)
	}

	updated, err := store.GetApplication(ctx, app.ID)
	if err != nil {
		t.Fatalf("get application: %v", err)
	}
	if updated.CurrentStage != domain.StageScreening {
		t.Errorf("application.CurrentStage = %s, want %s (stage event and application disagree)", updated.CurrentStage, domain.StageScreening)
	}

	events, err := store.ListStageEvents(ctx, app.ID)
	if err != nil {
		t.Fatalf("list stage events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d stage events, want 1", len(events))
	}
}

func TestRecordStageEventRejectsInvalidTransition(t *testing.T) {
	store := requireStore(t)
	ctx := context.Background()

	app, err := store.CreateApplication(ctx, "Initech", "Staff Engineer", domain.SourceCompanySite, time.Now().UTC())
	if err != nil {
		t.Fatalf("create application: %v", err)
	}

	_, err = store.RecordStageEvent(ctx, app.ID, domain.StageApplied, domain.StageOffer, domain.DetectedViaManual, nil, nil, "")
	if _, ok := err.(domain.ErrInvalidTransition); !ok {
		t.Fatalf("got err = %v, want domain.ErrInvalidTransition", err)
	}

	unchanged, err := store.GetApplication(ctx, app.ID)
	if err != nil {
		t.Fatalf("get application: %v", err)
	}
	if unchanged.CurrentStage != domain.StageApplied {
		t.Errorf("application.CurrentStage = %s, want unchanged %s after rejected transition", unchanged.CurrentStage, domain.StageApplied)
	}
}

func TestOAuthTokenRoundTrip(t *testing.T) {
	store := requireStore(t)
	ctx := context.Background()

	want := &oauth2.Token{
		AccessToken:  "ya29.test-access-token",
		RefreshToken: "1//test-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond),
	}

	if err := store.SaveToken(ctx, want); err != nil {
		t.Fatalf("save token: %v", err)
	}

	got, err := store.LoadToken(ctx)
	if err != nil {
		t.Fatalf("load token: %v", err)
	}

	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken || got.TokenType != want.TokenType {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if !got.Expiry.Equal(want.Expiry) {
		t.Errorf("got expiry %v, want %v", got.Expiry, want.Expiry)
	}

	// Saving again must overwrite the singleton row, not fail or duplicate.
	want.AccessToken = "ya29.rotated-access-token"
	if err := store.SaveToken(ctx, want); err != nil {
		t.Fatalf("save token (overwrite): %v", err)
	}
	got, err = store.LoadToken(ctx)
	if err != nil {
		t.Fatalf("load token after overwrite: %v", err)
	}
	if got.AccessToken != want.AccessToken {
		t.Errorf("got access token %q after overwrite, want %q", got.AccessToken, want.AccessToken)
	}
}
