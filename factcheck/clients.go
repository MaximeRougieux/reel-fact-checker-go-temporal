package factcheck

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"
)

// Clients are built once per Worker and injected into Activities. The mocks
// below return fixed text; each can later be swapped for a real provider
// (Whisper host, Haiku-class model, search API, Jev-style decision model).
type AudioFetcher interface {
	// Fetch returns a reference to stored audio, never the bytes.
	Fetch(ctx context.Context, reelURL string) (audioRef string, err error)
}

type Transcriber interface {
	Transcribe(ctx context.Context, audioRef string) (transcript string, err error)
}

type ClaimExtractor interface {
	Extract(ctx context.Context, transcript string) ([]Claim, error)
}

type Searcher interface {
	Search(ctx context.Context, claim Claim) ([]Evidence, error)
}

// ClaimJudge returns a typed decision with probabilities, not text.
type ClaimJudge interface {
	Judge(ctx context.Context, claim Claim, evidence []Evidence) (map[string]float64, error)
}

type Summarizer interface {
	Summarize(ctx context.Context, verdicts []Verdict) (string, error)
}

type MockFetcher struct{}

func (MockFetcher) Fetch(_ context.Context, reelURL string) (string, error) {
	return "mock://audio/" + shortcode(reelURL), nil
}

type MockTranscriber struct{}

func (MockTranscriber) Transcribe(_ context.Context, _ string) (string, error) {
	return "Coffee is the most traded commodity after oil. Because of that, coffee prices drive global politics.", nil
}

type MockExtractor struct{}

func (MockExtractor) Extract(_ context.Context, _ string) ([]Claim, error) {
	return []Claim{
		{Text: "Coffee is the most traded commodity after oil", Kind: "fact", TimestampSec: 2, Verifiable: true},
		{Text: "Coffee prices drive global politics", Kind: "implication",
			Premises: []string{"Coffee is the most traded commodity after oil"}, TimestampSec: 6, Verifiable: true},
	}, nil
}

type MockSearcher struct{}

func (MockSearcher) Search(_ context.Context, c Claim) ([]Evidence, error) {
	return []Evidence{
		{Source: "mock://source/1", Snippet: fmt.Sprintf("Mock evidence about %q.", c.Text)},
		{Source: "mock://source/2", Snippet: fmt.Sprintf("More mock evidence about %q.", c.Text)},
	}, nil
}

type MockJudge struct{}

// Deterministic by claim text so runs are repeatable.
func (MockJudge) Judge(_ context.Context, c Claim, _ []Evidence) (map[string]float64, error) {
	h := fnv.New32a()
	h.Write([]byte(c.Text))
	if h.Sum32()%2 == 0 {
		return map[string]float64{Supported: 0.82, Refuted: 0.06, Insufficient: 0.12}, nil
	}
	return map[string]float64{Supported: 0.20, Refuted: 0.15, Insufficient: 0.65}, nil
}

type MockSummarizer struct{}

func (MockSummarizer) Summarize(_ context.Context, vs []Verdict) (string, error) {
	var b strings.Builder
	for _, v := range vs {
		fmt.Fprintf(&b, "- [%s, %.0f%%] (%s) %s\n", v.Label, v.Confidence*100, v.Claim.Kind, v.Claim.Text)
	}
	return b.String(), nil
}
