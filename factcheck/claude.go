package factcheck

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

// newClaudeClient reads ANTHROPIC_API_KEY from the environment. The SDK's own
// retries are disabled: Temporal owns retry.
func newClaudeClient() anthropic.Client {
	return anthropic.NewClient(option.WithMaxRetries(0))
}

// structuredCall sends one user message and returns the JSON text constrained
// by schema.
func structuredCall(ctx context.Context, c anthropic.Client, model, system string, schema map[string]any, user string) (string, error) {
	resp, err := c.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 4096,
		System:    []anthropic.TextBlockParam{{Text: system}},
		OutputConfig: anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffortLow,
			Format: anthropic.JSONOutputFormatParam{Schema: schema},
		},
		Messages: []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(user))},
	})
	if err != nil {
		return "", err
	}
	if resp.StopReason == anthropic.StopReasonRefusal || resp.StopReason == anthropic.StopReasonMaxTokens {
		return "", fmt.Errorf("model stopped: %s", resp.StopReason)
	}
	var text string
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			text += t.Text
		}
	}
	return text, nil
}

// textCall sends one user message and returns the plain text reply.
func textCall(ctx context.Context, c anthropic.Client, model, system, user string) (string, error) {
	resp, err := c.Messages.New(ctx, anthropic.MessageNewParams{
		Model:        anthropic.Model(model),
		MaxTokens:    8192,
		System:       []anthropic.TextBlockParam{{Text: system}},
		OutputConfig: anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffortLow},
		Messages:     []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(user))},
	})
	if err != nil {
		return "", err
	}
	if resp.StopReason == anthropic.StopReasonRefusal || resp.StopReason == anthropic.StopReasonMaxTokens {
		return "", fmt.Errorf("model stopped: %s", resp.StopReason)
	}
	var text string
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			text += t.Text
		}
	}
	return text, nil
}
