package classify

import (
	"strings"

	"github.com/prashantkoirala465/sift/internal/domain"
)

// atsDomains are sender domains for the ATS platforms most job applications
// route through. Recognizing one raises confidence in whatever keyword
// match fires alongside it, and on its own is a (weak) signal the mail is
// application-related at all.
var atsDomains = []string{
	"greenhouse.io",
	"lever.co",
	"myworkday.com",
	"smartrecruiters.com",
	"icims.com",
	"workable.com",
	"ashbyhq.com",
	"bamboohr.com",
	"jobvite.com",
	"taleo.net",
	"successfactors.com",
}

// IsKnownATSDomain reports whether fromDomain belongs to a shared ATS
// platform. Exported because the matcher needs the same fact for a
// different reason: a domain shared across many companies is useless as a
// per-company matching signal, however useful it is as a classification
// signal here.
func IsKnownATSDomain(fromDomain string) bool {
	fromDomain = strings.ToLower(fromDomain)
	for _, suffix := range atsDomains {
		if fromDomain == suffix || strings.HasSuffix(fromDomain, "."+suffix) {
			return true
		}
	}
	return false
}

// Phrase lists are checked in this order deliberately: a rejection often
// still contains "thank you for applying" boilerplate near the top, so
// rejection must be checked before confirmation or it never matches.
var (
	rejectionPhrases = []string{
		"unfortunately", "not moving forward", "decided not to move forward",
		"other candidates", "not been selected", "will not be moving forward",
		"regret to inform", "pursue other candidates", "not selected for this position",
	}
	offerPhrases = []string{
		"pleased to offer", "excited to offer", "offer of employment", "job offer",
	}
	assessmentPhrases = []string{
		"coding challenge", "take-home", "take home assignment", "online assessment",
		"technical assessment", "complete the following assessment",
	}
	interviewPhrases = []string{
		"schedule a call", "schedule an interview", "phone screen", "next steps",
		"would like to interview", "set up a time to chat", "book a time",
	}
	confirmationPhrases = []string{
		"received your application", "thank you for applying", "application has been received",
		"thanks for your interest",
	}
)

// RuleClassifier is a pure, deterministic classifier: sender domain plus
// keyword heuristics on the subject and snippet. It never calls out to
// anything and never fails.
type RuleClassifier struct{}

func (RuleClassifier) Classify(in Input) Result {
	text := strings.ToLower(in.Subject + " " + in.Snippet)
	knownATS := IsKnownATSDomain(in.FromDomain)

	switch {
	case matchesAny(text, rejectionPhrases):
		return confidentResult(domain.LabelRejection, knownATS)
	case matchesAny(text, offerPhrases):
		return confidentResult(domain.LabelOffer, knownATS)
	case matchesAny(text, assessmentPhrases):
		return confidentResult(domain.LabelAssessment, knownATS)
	case matchesAny(text, interviewPhrases):
		return confidentResult(domain.LabelInterview, knownATS)
	case matchesAny(text, confirmationPhrases):
		return confidentResult(domain.LabelConfirmation, knownATS)
	default:
		conf := 0.0
		if knownATS {
			conf = 0.3 // recognizably ATS mail, but we don't know what kind
		}
		return Result{Label: domain.LabelUnclassified, Confidence: conf, Source: domain.ClassificationSourceRule}
	}
}

func confidentResult(label domain.ClassifiedLabel, knownATS bool) Result {
	conf := 0.85
	if knownATS {
		conf = 0.97
	}
	return Result{Label: label, Confidence: conf, Source: domain.ClassificationSourceRule}
}

func matchesAny(text string, phrases []string) bool {
	for _, p := range phrases {
		if strings.Contains(text, p) {
			return true
		}
	}
	return false
}
