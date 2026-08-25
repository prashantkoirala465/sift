package domain

import (
	"time"

	"github.com/google/uuid"
)

// Source is how the user applied to a role.
type Source string

const (
	SourceLinkedIn    Source = "linkedin"
	SourceReferral    Source = "referral"
	SourceCompanySite Source = "company_site"
	SourceJobBoard    Source = "job_board"
	SourceOther       Source = "other"
)

func (s Source) Valid() bool {
	switch s {
	case SourceLinkedIn, SourceReferral, SourceCompanySite, SourceJobBoard, SourceOther:
		return true
	default:
		return false
	}
}

type Application struct {
	ID           uuid.UUID
	Company      string
	RoleTitle    string
	Source       Source
	AppliedDate  time.Time
	CurrentStage Stage
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// DetectedVia records whether a StageEvent came from the email pipeline or
// was entered by hand.
type DetectedVia string

const (
	DetectedViaEmailAuto DetectedVia = "email_auto"
	DetectedViaManual    DetectedVia = "manual"
)

// StageEvent is an immutable audit record of one pipeline transition.
// Applications.CurrentStage is a derived, denormalized read of the latest
// StageEvent -- kept in sync in the same transaction that inserts the
// event, never written independently.
type StageEvent struct {
	ID            uuid.UUID
	ApplicationID uuid.UUID
	FromStage     Stage
	ToStage       Stage
	DetectedVia   DetectedVia
	SourceEmailID *uuid.UUID // set only when DetectedVia == DetectedViaEmailAuto
	Confidence    *float64   // set only when DetectedVia == DetectedViaEmailAuto
	Note          string
	OccurredAt    time.Time
}
