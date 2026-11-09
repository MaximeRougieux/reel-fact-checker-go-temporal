package factcheck

import (
	"context"
	"fmt"
)

// LLM is the provider-agnostic text model behind the extractor, the judge and
// the summarizer. A provider only has to answer two kinds of request.
type LLM interface {
	// Structured sends one user message and returns the JSON text constrained
	// by schema.
	Structured(ctx context.Context, system string, schema map[string]any, user string) (string, error)
	// Text sends one user message and returns the plain text reply.
	Text(ctx context.Context, system, user string) (string, error)
}

const (
	ProviderClaude  = "claude"
	ProviderMistral = "mistral"
)

// NewLLM builds the client for a provider. API keys are read from the
// environment by the provider (ANTHROPIC_API_KEY, MISTRAL_API_KEY). Providers
// do not retry on their own: Temporal owns retry.
func NewLLM(provider, model string) (LLM, error) {
	switch provider {
	case ProviderClaude:
		return NewClaudeLLM(model), nil
	case ProviderMistral:
		return NewMistralLLM(model), nil
	default:
		return nil, fmt.Errorf("unknown LLM provider %q (want %q or %q)", provider, ProviderClaude, ProviderMistral)
	}
}
