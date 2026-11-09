// Package llmconfig holds the environment-driven settings for the text-model
// providers (Claude, Mistral).
package llmconfig

import (
	"fmt"
	"os"
	"strings"

	"reel-fact-checker-go-temporal/factcheck"
	"reel-fact-checker-go-temporal/internal/config"
	"reel-fact-checker-go-temporal/internal/llmconfig/claude"
	"reel-fact-checker-go-temporal/internal/llmconfig/mistral"
)

// NewLLM builds the client for a provider ("claude" or "mistral"). API keys
// are read from the environment by the provider (ANTHROPIC_API_KEY,
// MISTRAL_API_KEY). Providers do not retry on their own: Temporal owns retry.
func NewLLM(provider, model string) (factcheck.LLM, error) {
	switch provider {
	case "claude":
		return claude.New(model), nil
	case "mistral":
		return mistral.New(model), nil
	default:
		return nil, fmt.Errorf("unknown LLM provider %q (want \"claude\" or \"mistral\")", provider)
	}
}

// Provider picks the text-model provider: LLM_PROVIDER if set, otherwise
// whichever API key is present. Empty means no provider is configured and the
// Worker uses mocks.
func Provider() string {
	if p := os.Getenv("LLM_PROVIDER"); p != "" {
		return strings.ToLower(p)
	}
	switch {
	case os.Getenv("ANTHROPIC_API_KEY") != "":
		return "claude"
	case os.Getenv("MISTRAL_API_KEY") != "":
		return "mistral"
	}
	return ""
}

// KeyVar is the environment variable holding a provider's API key.
func KeyVar(provider string) string {
	if provider == "mistral" {
		return "MISTRAL_API_KEY"
	}
	return "ANTHROPIC_API_KEY"
}

// ExtractorModel is the model used to extract claims.
func ExtractorModel() string {
	if Provider() == "mistral" {
		return config.Get("MISTRAL_MODEL", "mistral-small-latest")
	}
	return config.Get("ANTHROPIC_MODEL", "claude-haiku-5-5")
}

// JudgeModel is the model used to judge claims and write the summary. It
// defaults to the extractor model.
func JudgeModel() string {
	if Provider() == "mistral" {
		return config.Get("MISTRAL_JUDGE_MODEL", ExtractorModel())
	}
	return config.Get("ANTHROPIC_JUDGE_MODEL", ExtractorModel())
}
