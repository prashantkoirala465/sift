package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/prashantkoirala465/sift/internal/domain"
	"github.com/prashantkoirala465/sift/internal/match"
)

func registerReviewRoutes(mux *http.ServeMux, deps Deps) {
	mux.HandleFunc("GET /api/review-queue", handleListReviewQueue(deps))
	mux.HandleFunc("POST /api/review-queue/{id}/confirm", handleConfirmReview(deps))
	mux.HandleFunc("POST /api/review-queue/{id}/ignore", handleIgnoreReview(deps))
}

type reviewItemResponse struct {
	ID                       string   `json:"id"`
	FromAddress              string   `json:"from_address"`
	FromDomain               string   `json:"from_domain"`
	Subject                  string   `json:"subject"`
	Snippet                  string   `json:"snippet"`
	ReceivedAt               string   `json:"received_at"`
	ClassifiedLabel          *string  `json:"classified_label,omitempty"`
	ClassificationConfidence *float64 `json:"classification_confidence,omitempty"`
	SuggestedApplicationID   *string  `json:"suggested_application_id,omitempty"`
	MatchConfidence          *float64 `json:"match_confidence,omitempty"`
}

func toReviewItemResponse(m domain.EmailMessage) reviewItemResponse {
	resp := reviewItemResponse{
		ID:                       m.ID.String(),
		FromAddress:              m.FromAddress,
		FromDomain:               m.FromDomain,
		Subject:                  m.Subject,
		Snippet:                  m.Snippet,
		ReceivedAt:               m.ReceivedAt.Format(time.RFC3339),
		ClassificationConfidence: m.ClassificationConfidence,
		MatchConfidence:          m.MatchConfidence,
	}
	if m.ClassifiedLabel != nil {
		label := string(*m.ClassifiedLabel)
		resp.ClassifiedLabel = &label
	}
	if m.MatchedApplicationID != nil {
		id := m.MatchedApplicationID.String()
		resp.SuggestedApplicationID = &id
	}
	return resp
}

// review_status=pending covers both "no match found" and "a low-confidence
// guess exists" -- toReviewItemResponse surfaces the guess when there is
// one so the UI can offer it as a one-click confirm.
func handleListReviewQueue(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := deps.Store.ListEmailMessagesByReviewStatus(r.Context(), domain.ReviewStatusPending)
		if err != nil {
			writeInternalError(w, "list review queue", err)
			return
		}
		out := make([]reviewItemResponse, 0, len(items))
		for _, m := range items {
			out = append(out, toReviewItemResponse(m))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

type confirmReviewRequest struct {
	ApplicationID string `json:"application_id"`
}

// handleConfirmReview links an email to an application a human picked --
// either confirming the matcher's own guess or overriding it -- then
// applies the same implied-stage-transition logic the automatic path uses,
// now as a human-authorized (DetectedViaManual) event.
func handleConfirmReview(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		emailID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid review item id", http.StatusBadRequest)
			return
		}

		var req confirmReviewRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		applicationID, err := uuid.Parse(req.ApplicationID)
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
			writeInternalError(w, "confirm review match", err)
			return
		}

		if msg.ClassifiedLabel != nil {
			confidence := 1.0
			if _, err := match.ApplyImpliedTransition(r.Context(), deps.Store, applicationID, *msg.ClassifiedLabel, domain.DetectedViaManual, &emailID, &confidence); err != nil {
				writeInternalError(w, "apply confirmed transition", err)
				return
			}
		}

		w.WriteHeader(http.StatusNoContent)
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
			writeInternalError(w, "ignore review item", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
