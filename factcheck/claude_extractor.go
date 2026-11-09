package factcheck

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go/v2"
)

//go:embed prompts/extract_claims.system.md
var extractSystemPrompt string

//go:embed prompts/extract_claims.schema.json
var extractSchemaJSON []byte

// ClaudeExtractor extracts claims with the Claude API. The SDK's own retries
// are disabled: Temporal owns retry.
type ClaudeExtractor struct {
	client anthropic.Client
	model  string
	schema map[string]any
}

// NewClaudeExtractor reads ANTHROPIC_API_KEY from the environment.
func NewClaudeExtractor(model string) (*ClaudeExtractor, error) {
	var schema map[string]any
	if err := json.Unmarshal(extractSchemaJSON, &schema); err != nil {
		return nil, fmt.Errorf("bad extraction schema: %w", err)
	}
	return &ClaudeExtractor{
		client: newClaudeClient(),
		model:  model,
		schema: schema,
	}, nil
}

func (e *ClaudeExtractor) Extract(ctx context.Context, transcript string) ([]Claim, error) {
	text, err := structuredCall(ctx, e.client, e.model, extractSystemPrompt, e.schema,
		"<transcript>\n"+transcript+"\n</transcript>")
	if err != nil {
		return nil, err
	}
	var out struct {
		Claims []struct {
			Text         string   `json:"text"`
			Kind         string   `json:"kind"`
			Premises     []string `json:"premises"`
			TimestampSec int      `json:"timestamp_sec"`
			Verifiable   bool     `json:"verifiable"`
		} `json:"claims"`
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return nil, fmt.Errorf("invalid extraction output: %w", err)
	}
	claims := make([]Claim, len(out.Claims))
	for i, c := range out.Claims {
		claims[i] = Claim{Text: c.Text, Kind: c.Kind, Premises: c.Premises, TimestampSec: c.TimestampSec, Verifiable: c.Verifiable}
	}
	return claims, nil
}
