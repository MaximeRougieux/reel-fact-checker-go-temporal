package factcheck

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

// ClaudeLLM is the Anthropic provider. It reads ANTHROPIC_API_KEY from the
// environment. The SDK's own retries are disabled: Temporal owns retry.
type ClaudeLLM struct {
	client anthropic.Client
	model  string
}

func NewClaudeLLM(model string) *ClaudeLLM {
	return &ClaudeLLM{client: anthropic.NewClient(option.WithMaxRetries(0)), model: model}
}

func (c *ClaudeLLM) Structured(ctx context.Context, system string, schema map[string]any, user string) (string, error) {
	return c.call(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: 4096,
		System:    []anthropic.TextBlockParam{{Text: system}},
		OutputConfig: anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffortLow,
			Format: anthropic.JSONOutputFormatParam{Schema: schema},
		},
		Messages: []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(user))},
	})
}

func (c *ClaudeLLM) Text(ctx context.Context, system, user string) (string, error) {
	return c.call(ctx, anthropic.MessageNewParams{
		Model:        anthropic.Model(c.model),
		MaxTokens:    8192,
		System:       []anthropic.TextBlockParam{{Text: system}},
		OutputConfig: anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffortLow},
		Messages:     []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(user))},
	})
}

func (c *ClaudeLLM) call(ctx context.Context, p anthropic.MessageNewParams) (string, error) {
	resp, err := c.client.Messages.New(ctx, p)
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
