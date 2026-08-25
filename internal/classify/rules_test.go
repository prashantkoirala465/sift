package classify

import (
	"testing"

	"github.com/prashantkoirala465/sift/internal/domain"
)

func TestRuleClassifierLabels(t *testing.T) {
	cases := []struct {
		name  string
		in    Input
		label domain.ClassifiedLabel
	}{
		{
			"rejection with ATS domain",
			Input{Subject: "Update on your application", Snippet: "Unfortunately, we've decided to move forward with other candidates.", FromDomain: "greenhouse.io"},
			domain.LabelRejection,
		},
		{
			"rejection wins over confirmation boilerplate",
			Input{Subject: "Your application", Snippet: "Thank you for applying. Unfortunately, we will not be moving forward.", FromDomain: "mail.lever.co"},
			domain.LabelRejection,
		},
		{
			"offer",
			Input{Subject: "We'd like to extend an offer", Snippet: "We are pleased to offer you the position.", FromDomain: "acme.com"},
			domain.LabelOffer,
		},
		{
			"assessment",
			Input{Subject: "Next step: coding challenge", Snippet: "Please complete the following assessment within 5 days.", FromDomain: "hackerrank.com"},
			domain.LabelAssessment,
		},
		{
			"interview",
			Input{Subject: "Let's schedule a call", Snippet: "Would like to interview you next week.", FromDomain: "acme.com"},
			domain.LabelInterview,
		},
		{
			"confirmation",
			Input{Subject: "Application received", Snippet: "Thank you for applying to Acme Corp.", FromDomain: "myworkday.com"},
			domain.LabelConfirmation,
		},
		{
			"unclassified, unknown domain",
			Input{Subject: "Newsletter", Snippet: "Check out our latest blog post.", FromDomain: "substack.com"},
			domain.LabelUnclassified,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RuleClassifier{}.Classify(tc.in)
			if got.Label != tc.label {
				t.Errorf("Label = %s, want %s", got.Label, tc.label)
			}
			if got.Source != domain.ClassificationSourceRule {
				t.Errorf("Source = %s, want %s", got.Source, domain.ClassificationSourceRule)
			}
		})
	}
}

func TestRuleClassifierConfidenceHigherWithKnownATSDomain(t *testing.T) {
	base := Input{Subject: "Unfortunately", Snippet: "not moving forward", FromDomain: "random-startup.com"}
	known := base
	known.FromDomain = "greenhouse.io"

	baseResult := RuleClassifier{}.Classify(base)
	knownResult := RuleClassifier{}.Classify(known)

	if knownResult.Confidence <= baseResult.Confidence {
		t.Errorf("expected known-ATS confidence (%v) > unknown-domain confidence (%v)", knownResult.Confidence, baseResult.Confidence)
	}
}

func TestRuleClassifierUnclassifiedLowConfidenceOnUnknownDomain(t *testing.T) {
	got := RuleClassifier{}.Classify(Input{Subject: "Hi", Snippet: "how are you", FromDomain: "gmail.com"})
	if got.Label != domain.LabelUnclassified {
		t.Fatalf("Label = %s, want %s", got.Label, domain.LabelUnclassified)
	}
	if got.Confidence != 0 {
		t.Errorf("Confidence = %v, want 0 for an unrecognized domain with no keyword match", got.Confidence)
	}
}
