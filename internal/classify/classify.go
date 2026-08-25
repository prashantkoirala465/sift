// Package classify decides what an email is about. Two tiers: a free,
// deterministic rule pass first, and an LLM consulted only when the rules
// aren't confident (see TieredClassifier). Classification never decides
// what happens to an application on its own -- that's the matcher and the
// state machine's job, downstream of this package.
package classify

import (
	"context"

	"github.com/prashantkoirala465/sift/internal/domain"
)

// Input is the slice of an email classification actually looks at.
type Input struct {
	Subject    string
	Snippet    string
	FromDomain string
}

type Result struct {
	Label      domain.ClassifiedLabel
	Confidence float64
	Source     domain.ClassificationSource
}

// Classifier is implemented by both tiers, so TieredClassifier can wrap
// either interchangeably and tests can substitute a fake.
type Classifier interface {
	Classify(ctx context.Context, in Input) (Result, error)
}
