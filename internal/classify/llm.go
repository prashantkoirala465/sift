package classify

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/prashantkoirala465/sift/internal/domain"
)

// llmModel is deliberately the cheap, fast tier -- classification is a
// narrow single-label decision on a few hundred characters, consulted only
// for the fraction of mail the free rule pass can't place. That doesn't
// need Sift's most capable model available.
const llmModel = "claude-haiku-4-5-20251001"

var classifyTool = anthropic.ToolParam{
	Name:        "classify_email",
	Description: anthropic.String("Classify a job-application-related email by what it's about."),
	InputSchema: anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"label": map[string]any{
				"type":        "string",
				"enum":        []string{"confirmation", "rejection", "interview", "offer", "assessment", "other"},
				"description": "The single best label for this email.",
			},
			"confidence": map[string]any{
				"type":        "number",
				"description": "Confidence in this label, from 0 to 1.",
			},
		},
		Required: []string{"label", "confidence"},
		ExtraFields: map[string]any{
			"additionalProperties": false,
		},
	},
	Strict: anthropic.Bool(true),
}

// LLMClassifier is the fallback tier, consulted only when RuleClassifier
// isn't confident. Forcing tool use means the response is always
// structured -- never freeform text that needs its own parsing and error
// handling.
type LLMClassifier struct {
	client anthropic.Client
}

func NewLLMClassifier(apiKey string) LLMClassifier {
	var opts []option.RequestOption
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	return LLMClassifier{client: anthropic.NewClient(opts...)}
}

func (c LLMClassifier) Classify(ctx context.Context, in Input) (Result, error) {
	prompt := fmt.Sprintf("Subject: %s\nSender domain: %s\nSnippet: %s", in.Subject, in.FromDomain, in.Snippet)

	resp, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     llmModel,
		MaxTokens: 256,
		System: []anthropic.TextBlockParam{
			{Text: "You classify job application emails. Always call classify_email exactly once with your best label."},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
		Tools:      []anthropic.ToolUnionParam{{OfTool: &classifyTool}},
		ToolChoice: anthropic.ToolChoiceParamOfTool("classify_email"),
	})
	if err != nil {
		return Result{}, fmt.Errorf("classify via llm: %w", err)
	}

	for _, block := range resp.Content {
		tu, ok := block.AsAny().(anthropic.ToolUseBlock)
		if !ok {
			continue
		}

		var parsed struct {
			Label      string  `json:"label"`
			Confidence float64 `json:"confidence"`
		}
		if err := json.Unmarshal([]byte(tu.JSON.Input.Raw()), &parsed); err != nil {
			return Result{}, fmt.Errorf("parse llm classification: %w", err)
		}

		return Result{
			Label:      domain.ClassifiedLabel(parsed.Label),
			Confidence: parsed.Confidence,
			Source:     domain.ClassificationSourceLLM,
		}, nil
	}

	return Result{}, fmt.Errorf("llm response had no tool_use block (stop_reason=%s)", resp.StopReason)
}
