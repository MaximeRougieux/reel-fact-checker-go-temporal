package factcheck

import "context"

// LLM is the provider-agnostic text model behind the extractor, the judge and
// the summarizer. A provider only has to answer two kinds of request.
type LLM interface {
	// Structured sends one user message and returns the JSON text constrained
	// by schema.
	Structured(ctx context.Context, system string, schema map[string]any, user string) (string, error)
	// Text sends one user message and returns the plain text reply.
	Text(ctx context.Context, system, user string) (string, error)
}
