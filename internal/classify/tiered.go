package classify

import (
	"context"
	"log/slog"
)

// confidenceThreshold: below this, RuleClassifier's result isn't trusted
// and the LLM fallback is consulted instead.
const confidenceThreshold = 0.8

// TieredClassifier tries the rule pass first; only mail the rules can't
// confidently place is sent to the LLM. Cheap, deterministic
// classifications never touch the network -- only genuinely ambiguous ones
// cost a token.
type TieredClassifier struct {
	rules  RuleClassifier
	llm    Classifier // nil disables the fallback tier entirely
	logger *slog.Logger
}

// NewTieredClassifier builds the two-tier classifier. Pass a nil llm to
// run rules-only (e.g. no Anthropic API key configured) -- everything
// still classifies, just with lower confidence on ambiguous mail.
func NewTieredClassifier(llm Classifier, logger *slog.Logger) *TieredClassifier {
	return &TieredClassifier{rules: RuleClassifier{}, llm: llm, logger: logger}
}

func (t *TieredClassifier) Classify(ctx context.Context, in Input) Result {
	ruleResult := t.rules.Classify(in)
	if ruleResult.Confidence >= confidenceThreshold || t.llm == nil {
		return ruleResult
	}

	llmResult, err := t.llm.Classify(ctx, in)
	if err != nil {
		t.logger.Warn("llm classification failed, keeping rule result", "error", err)
		return ruleResult
	}
	return llmResult
}
