package domain

import (
	"time"

	"github.com/google/uuid"
)

// ClassifiedLabel is what the classification pipeline decided an email is
// about. It's a signal for matching, not itself a Stage -- a "rejection"
// email still has to be matched to an Application before it can produce a
// StageEvent moving that application to StageRejected.
type ClassifiedLabel string

const (
	LabelConfirmation ClassifiedLabel = "confirmation"
	LabelRejection    ClassifiedLabel = "rejection"
	LabelInterview    ClassifiedLabel = "interview"
	LabelOffer        ClassifiedLabel = "offer"
	LabelAssessment   ClassifiedLabel = "assessment"
	LabelOther        ClassifiedLabel = "other"
	LabelUnclassified ClassifiedLabel = "unclassified"
)

// ClassificationSource is which tier of the classifier produced the label.
type ClassificationSource string

const (
	ClassificationSourceRule ClassificationSource = "rule"
	ClassificationSourceLLM  ClassificationSource = "llm"
)

// ReviewStatus tracks whether an email still needs a human decision before
// it affects any Application.
type ReviewStatus string

const (
	ReviewStatusPending ReviewStatus = "pending" // awaiting classification and/or match
	ReviewStatusMatched ReviewStatus = "matched" // linked to an application, event recorded
	ReviewStatusIgnored ReviewStatus = "ignored" // user dismissed it, or it's confidently irrelevant
)

type EmailMessage struct {
	ID                       uuid.UUID
	GmailMessageID           string
	GmailThreadID            string
	FromAddress              string
	FromDomain               string
	Subject                  string
	ReceivedAt               time.Time
	ClassifiedLabel          *ClassifiedLabel
	ClassificationConfidence *float64
	ClassificationSource     *ClassificationSource
	MatchedApplicationID     *uuid.UUID
	MatchConfidence          *float64
	ReviewStatus             ReviewStatus
	ProcessedAt              *time.Time
	CreatedAt                time.Time
}

// SyncState tracks the single Gmail sync checkpoint. An empty
// LastHistoryID means no sync has completed yet -- the next tick does a
// bounded historical backfill instead of an incremental History.List call.
type SyncState struct {
	LastHistoryID string
	LastSyncedAt  *time.Time
}
