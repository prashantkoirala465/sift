package web

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/prashantkoirala465/sift/internal/domain"
	"github.com/prashantkoirala465/sift/internal/gmail"
	"github.com/prashantkoirala465/sift/internal/match"
)

const dateLayout = "2006-01-02"

// Deps are the dependencies the web handlers need.
type Deps struct {
	Store      Store
	TokenStore gmail.TokenStore
}

// RegisterRoutes adds Sift's HTML UI to mux, alongside the JSON API and
// static assets.
func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	mux.Handle("GET /static/", StaticHandler())

	mux.HandleFunc("GET /{$}", handleDashboard(deps))
	mux.HandleFunc("POST /applications", handleCreateApplication(deps))
	mux.HandleFunc("GET /applications/{id}", handleApplicationDetail(deps))
	mux.HandleFunc("POST /applications/{id}/stage", handleRecordStage(deps))

	mux.HandleFunc("GET /review-queue", handleReviewQueue(deps))
	mux.HandleFunc("POST /review-queue/{id}/confirm", handleConfirmReview(deps))
	mux.HandleFunc("POST /review-queue/{id}/ignore", handleIgnoreReview(deps))
}

func gmailConnected(ctx context.Context, tokenStore gmail.TokenStore) bool {
	if tokenStore == nil {
		return false
	}
	_, err := tokenStore.LoadToken(ctx)
	if err != nil && !errors.Is(err, gmail.ErrNoToken) {
		slog.Warn("check gmail connection status failed", "error", err)
	}
	return err == nil
}

type applicationCard struct {
	ID        string
	Company   string
	RoleTitle string
	Stage     string
	Terminal  bool
}

type stageColumn struct {
	Title        string
	Applications []applicationCard
}

var boardColumns = []struct {
	Title  string
	Stages []domain.Stage
}{
	{"Applied", []domain.Stage{domain.StageApplied}},
	{"Screening", []domain.Stage{domain.StageScreening}},
	{"Interview", []domain.Stage{domain.StageInterview}},
	{"Offer", []domain.Stage{domain.StageOffer}},
	{"Closed", []domain.Stage{domain.StageAccepted, domain.StageDeclined, domain.StageRejected, domain.StageWithdrawn}},
}

func handleDashboard(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apps, err := deps.Store.ListApplications(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			slog.Error("list applications failed", "error", err)
			return
		}

		byStage := make(map[domain.Stage][]domain.Application)
		for _, a := range apps {
			byStage[a.CurrentStage] = append(byStage[a.CurrentStage], a)
		}

		columns := make([]stageColumn, 0, len(boardColumns))
		for _, col := range boardColumns {
			var cards []applicationCard
			for _, stage := range col.Stages {
				for _, a := range byStage[stage] {
					cards = append(cards, applicationCard{
						ID:        a.ID.String(),
						Company:   a.Company,
						RoleTitle: a.RoleTitle,
						Stage:     string(a.CurrentStage),
						Terminal:  len(col.Stages) > 1,
					})
				}
			}
			columns = append(columns, stageColumn{Title: col.Title, Applications: cards})
		}

		render(w, "dashboard", struct {
			GmailConnected bool
			Columns        []stageColumn
		}{
			GmailConnected: gmailConnected(r.Context(), deps.TokenStore),
			Columns:        columns,
		})
	}
}

func handleCreateApplication(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		source := domain.Source(r.FormValue("source"))
		if !source.Valid() {
			http.Error(w, "invalid source", http.StatusBadRequest)
			return
		}
		appliedDate, err := time.Parse(dateLayout, r.FormValue("applied_date"))
		if err != nil {
			http.Error(w, "invalid applied_date", http.StatusBadRequest)
			return
		}
		company, roleTitle := r.FormValue("company"), r.FormValue("role_title")
		if company == "" || roleTitle == "" {
			http.Error(w, "company and role are required", http.StatusBadRequest)
			return
		}

		if _, err := deps.Store.CreateApplication(r.Context(), company, roleTitle, source, appliedDate); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			slog.Error("create application failed", "error", err)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func handleApplicationDetail(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid application id", http.StatusBadRequest)
			return
		}

		app, err := deps.Store.GetApplication(r.Context(), id)
		if err != nil {
			http.Error(w, "application not found", http.StatusNotFound)
			return
		}
		events, err := deps.Store.ListStageEvents(r.Context(), id)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			slog.Error("list stage events failed", "error", err)
			return
		}

		// Only offer transitions the state machine actually allows -- the
		// UI shouldn't let someone attempt a move it already knows will
		// 409.
		var nextStages []string
		for _, s := range domain.AllStages {
			if domain.CanTransition(app.CurrentStage, s) {
				nextStages = append(nextStages, string(s))
			}
		}

		type timelineEvent struct {
			FromStage   string
			ToStage     string
			DetectedVia string
			Note        string
			OccurredAt  string
		}
		eventViews := make([]timelineEvent, 0, len(events))
		for _, e := range events {
			eventViews = append(eventViews, timelineEvent{
				FromStage:   string(e.FromStage),
				ToStage:     string(e.ToStage),
				DetectedVia: string(e.DetectedVia),
				Note:        e.Note,
				OccurredAt:  e.OccurredAt.Format(time.RFC822),
			})
		}

		render(w, "application_detail", struct {
			ID           string
			Company      string
			RoleTitle    string
			Source       string
			AppliedDate  string
			CurrentStage string
			NextStages   []string
			Events       []timelineEvent
		}{
			ID:           app.ID.String(),
			Company:      app.Company,
			RoleTitle:    app.RoleTitle,
			Source:       string(app.Source),
			AppliedDate:  app.AppliedDate.Format(dateLayout),
			CurrentStage: string(app.CurrentStage),
			NextStages:   nextStages,
			Events:       eventViews,
		})
	}
}

func handleRecordStage(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid application id", http.StatusBadRequest)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		toStage := domain.Stage(r.FormValue("to_stage"))
		if !toStage.Valid() {
			http.Error(w, "invalid to_stage", http.StatusBadRequest)
			return
		}

		app, err := deps.Store.GetApplication(r.Context(), id)
		if err != nil {
			http.Error(w, "application not found", http.StatusNotFound)
			return
		}

		_, err = deps.Store.RecordStageEvent(r.Context(), id, app.CurrentStage, toStage, domain.DetectedViaManual, nil, nil, r.FormValue("note"))
		if err != nil {
			var invalid domain.ErrInvalidTransition
			if errors.As(err, &invalid) {
				http.Error(w, invalid.Error(), http.StatusConflict)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			slog.Error("record stage event failed", "error", err)
			return
		}
		http.Redirect(w, r, "/applications/"+id.String(), http.StatusSeeOther)
	}
}

type reviewItem struct {
	ID                     string
	FromAddress            string
	Subject                string
	Snippet                string
	ClassifiedLabel        string
	SuggestedApplicationID string
}

type applicationOption struct {
	ID    string
	Label string
}

func handleReviewQueue(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := deps.Store.ListEmailMessagesByReviewStatus(r.Context(), domain.ReviewStatusPending)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			slog.Error("list review queue failed", "error", err)
			return
		}
		apps, err := deps.Store.ListApplications(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			slog.Error("list applications failed", "error", err)
			return
		}

		options := make([]applicationOption, 0, len(apps))
		for _, a := range apps {
			options = append(options, applicationOption{ID: a.ID.String(), Label: a.Company + " — " + a.RoleTitle})
		}

		views := make([]reviewItem, 0, len(items))
		for _, m := range items {
			item := reviewItem{ID: m.ID.String(), FromAddress: m.FromAddress, Subject: m.Subject, Snippet: m.Snippet}
			if m.ClassifiedLabel != nil {
				item.ClassifiedLabel = string(*m.ClassifiedLabel)
			}
			if m.MatchedApplicationID != nil {
				item.SuggestedApplicationID = m.MatchedApplicationID.String()
			}
			views = append(views, item)
		}

		render(w, "review_queue", struct {
			Items        []reviewItem
			Applications []applicationOption
		}{Items: views, Applications: options})
	}
}

func handleConfirmReview(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		emailID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid review item id", http.StatusBadRequest)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		applicationID, err := uuid.Parse(r.FormValue("application_id"))
		if err != nil {
			http.Error(w, "invalid application_id", http.StatusBadRequest)
			return
		}

		msg, err := deps.Store.GetEmailMessage(r.Context(), emailID)
		if err != nil {
			http.Error(w, "review item not found", http.StatusNotFound)
			return
		}
		if _, err := deps.Store.GetApplication(r.Context(), applicationID); err != nil {
			http.Error(w, "application not found", http.StatusNotFound)
			return
		}

		if err := deps.Store.SetEmailMatch(r.Context(), emailID, applicationID, 1.0, domain.ReviewStatusMatched); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			slog.Error("confirm review match failed", "error", err)
			return
		}
		if msg.ClassifiedLabel != nil {
			confidence := 1.0
			if _, err := match.ApplyImpliedTransition(r.Context(), deps.Store, applicationID, *msg.ClassifiedLabel, domain.DetectedViaManual, &emailID, &confidence); err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				slog.Error("apply confirmed transition failed", "error", err)
				return
			}
		}

		// hx-swap="outerHTML" against the row: an empty response removes it.
		w.WriteHeader(http.StatusOK)
	}
}

func handleIgnoreReview(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		emailID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid review item id", http.StatusBadRequest)
			return
		}
		if err := deps.Store.SetEmailReviewStatus(r.Context(), emailID, domain.ReviewStatusIgnored); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			slog.Error("ignore review item failed", "error", err)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
