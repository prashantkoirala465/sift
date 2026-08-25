package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/prashantkoirala465/sift/internal/domain"
)

const dateLayout = "2006-01-02"

func registerApplicationRoutes(mux *http.ServeMux, deps Deps) {
	mux.HandleFunc("POST /api/applications", handleCreateApplication(deps))
	mux.HandleFunc("GET /api/applications", handleListApplications(deps))
	mux.HandleFunc("GET /api/applications/{id}", handleGetApplication(deps))
	mux.HandleFunc("POST /api/applications/{id}/stage", handleRecordStage(deps))
}

type applicationResponse struct {
	ID           string    `json:"id"`
	Company      string    `json:"company"`
	RoleTitle    string    `json:"role_title"`
	Source       string    `json:"source"`
	AppliedDate  string    `json:"applied_date"`
	CurrentStage string    `json:"current_stage"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func toApplicationResponse(a domain.Application) applicationResponse {
	return applicationResponse{
		ID:           a.ID.String(),
		Company:      a.Company,
		RoleTitle:    a.RoleTitle,
		Source:       string(a.Source),
		AppliedDate:  a.AppliedDate.Format(dateLayout),
		CurrentStage: string(a.CurrentStage),
		CreatedAt:    a.CreatedAt,
		UpdatedAt:    a.UpdatedAt,
	}
}

type stageEventResponse struct {
	ID            string   `json:"id"`
	FromStage     string   `json:"from_stage"`
	ToStage       string   `json:"to_stage"`
	DetectedVia   string   `json:"detected_via"`
	SourceEmailID *string  `json:"source_email_id,omitempty"`
	Confidence    *float64 `json:"confidence,omitempty"`
	Note          string   `json:"note,omitempty"`
	OccurredAt    string   `json:"occurred_at"`
}

func toStageEventResponse(e domain.StageEvent) stageEventResponse {
	resp := stageEventResponse{
		ID:          e.ID.String(),
		FromStage:   string(e.FromStage),
		ToStage:     string(e.ToStage),
		DetectedVia: string(e.DetectedVia),
		Confidence:  e.Confidence,
		Note:        e.Note,
		OccurredAt:  e.OccurredAt.Format(time.RFC3339),
	}
	if e.SourceEmailID != nil {
		id := e.SourceEmailID.String()
		resp.SourceEmailID = &id
	}
	return resp
}

type createApplicationRequest struct {
	Company     string `json:"company"`
	RoleTitle   string `json:"role_title"`
	Source      string `json:"source"`
	AppliedDate string `json:"applied_date"`
}

func handleCreateApplication(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createApplicationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if req.Company == "" || req.RoleTitle == "" {
			http.Error(w, "company and role_title are required", http.StatusBadRequest)
			return
		}
		source := domain.Source(req.Source)
		if !source.Valid() {
			http.Error(w, "invalid source", http.StatusBadRequest)
			return
		}
		appliedDate, err := time.Parse(dateLayout, req.AppliedDate)
		if err != nil {
			http.Error(w, "applied_date must be YYYY-MM-DD", http.StatusBadRequest)
			return
		}

		app, err := deps.Store.CreateApplication(r.Context(), req.Company, req.RoleTitle, source, appliedDate)
		if err != nil {
			writeInternalError(w, "create application", err)
			return
		}
		writeJSON(w, http.StatusCreated, toApplicationResponse(app))
	}
}

func handleListApplications(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apps, err := deps.Store.ListApplications(r.Context())
		if err != nil {
			writeInternalError(w, "list applications", err)
			return
		}
		out := make([]applicationResponse, 0, len(apps))
		for _, a := range apps {
			out = append(out, toApplicationResponse(a))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

type applicationDetailResponse struct {
	applicationResponse
	StageEvents []stageEventResponse `json:"stage_events"`
}

func handleGetApplication(deps Deps) http.HandlerFunc {
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
			writeInternalError(w, "list stage events", err)
			return
		}

		eventResponses := make([]stageEventResponse, 0, len(events))
		for _, e := range events {
			eventResponses = append(eventResponses, toStageEventResponse(e))
		}
		writeJSON(w, http.StatusOK, applicationDetailResponse{
			applicationResponse: toApplicationResponse(app),
			StageEvents:         eventResponses,
		})
	}
}

type recordStageRequest struct {
	ToStage string `json:"to_stage"`
	Note    string `json:"note"`
}

func handleRecordStage(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid application id", http.StatusBadRequest)
			return
		}

		var req recordStageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		toStage := domain.Stage(req.ToStage)
		if !toStage.Valid() {
			http.Error(w, "invalid to_stage", http.StatusBadRequest)
			return
		}

		app, err := deps.Store.GetApplication(r.Context(), id)
		if err != nil {
			http.Error(w, "application not found", http.StatusNotFound)
			return
		}

		event, err := deps.Store.RecordStageEvent(r.Context(), id, app.CurrentStage, toStage, domain.DetectedViaManual, nil, nil, req.Note)
		if err != nil {
			var invalid domain.ErrInvalidTransition
			if errors.As(err, &invalid) {
				http.Error(w, invalid.Error(), http.StatusConflict)
				return
			}
			writeInternalError(w, "record stage event", err)
			return
		}
		writeJSON(w, http.StatusOK, toStageEventResponse(event))
	}
}
