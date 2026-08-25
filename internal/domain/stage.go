package domain

import "fmt"

// Stage is a position in an application's pipeline. Sift models multiple
// interview rounds as repeated StageEvents against a single StageInterview,
// not as distinct per-round stages -- round count varies per company and
// hardcoding a fixed number of interview stages doesn't hold up.
type Stage string

const (
	StageApplied   Stage = "applied"
	StageScreening Stage = "screening"
	StageInterview Stage = "interview"
	StageOffer     Stage = "offer"
	StageAccepted  Stage = "accepted"
	StageDeclined  Stage = "declined"
	StageRejected  Stage = "rejected"
	StageWithdrawn Stage = "withdrawn"
)

// AllStages lists every valid stage, in pipeline order. For callers that
// need to enumerate stages -- e.g. a UI offering only the valid next
// stages via CanTransition.
var AllStages = []Stage{
	StageApplied, StageScreening, StageInterview, StageOffer,
	StageAccepted, StageDeclined, StageRejected, StageWithdrawn,
}

func (s Stage) Valid() bool {
	switch s {
	case StageApplied, StageScreening, StageInterview, StageOffer,
		StageAccepted, StageDeclined, StageRejected, StageWithdrawn:
		return true
	default:
		return false
	}
}

// terminal stages have no outbound transitions.
func (s Stage) terminal() bool {
	switch s {
	case StageAccepted, StageDeclined, StageRejected, StageWithdrawn:
		return true
	default:
		return false
	}
}

// transitions maps each non-terminal stage to the set of stages it may move
// to. rejected and withdrawn are reachable from every non-terminal stage
// (a company can reject you, or you can withdraw, at any point before a
// final accept/decline) and are added programmatically in allowedFrom
// rather than repeated in every entry below.
var transitions = map[Stage][]Stage{
	StageApplied:   {StageScreening},
	StageScreening: {StageInterview},
	StageInterview: {StageInterview, StageOffer}, // StageInterview -> StageInterview: next round
	StageOffer:     {StageAccepted, StageDeclined},
}

// allowedFrom returns every stage reachable from s in one transition.
func allowedFrom(s Stage) []Stage {
	if s.terminal() {
		return nil
	}
	next := append([]Stage{}, transitions[s]...)
	return append(next, StageRejected, StageWithdrawn)
}

// CanTransition reports whether moving an application from `from` to `to`
// is a legal state-machine transition.
func CanTransition(from, to Stage) bool {
	for _, s := range allowedFrom(from) {
		if s == to {
			return true
		}
	}
	return false
}

// ErrInvalidTransition is returned by callers that enforce CanTransition and
// reject the write.
type ErrInvalidTransition struct {
	From, To Stage
}

func (e ErrInvalidTransition) Error() string {
	return fmt.Sprintf("invalid stage transition: %s -> %s", e.From, e.To)
}
