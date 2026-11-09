package factcheck

import (
	"context"
	_ "embed"
	"encoding/json"
	"sort"

	"github.com/anthropics/anthropic-sdk-go/v2"
)

//go:embed prompts/summarize.system.md
var summarizeSystemPrompt string

// ClaudeSummarizer drafts the final report. Labels and probabilities are
// computed upstream; the model only writes them up.
type ClaudeSummarizer struct {
	client anthropic.Client
	model  string
}

func NewClaudeSummarizer(model string) *ClaudeSummarizer {
	return &ClaudeSummarizer{client: newClaudeClient(), model: model}
}

func (s *ClaudeSummarizer) Summarize(ctx context.Context, verdicts []Verdict) (string, error) {
	type src struct {
		URL     string  `json:"url"`
		Score   float64 `json:"relevance"`
		Extract string  `json:"extract"`
	}
	type item struct {
		Claim         string             `json:"claim"`
		Kind          string             `json:"kind"`
		Label         string             `json:"label"`
		Probabilities map[string]float64 `json:"probabilities"`
		Sources       []src              `json:"sources"`
	}
	items := make([]item, len(verdicts))
	for i, v := range verdicts {
		ev := append([]Evidence(nil), v.Evidence...)
		sort.Slice(ev, func(a, b int) bool { return ev[a].Score > ev[b].Score })
		it := item{Claim: v.Claim.Text, Kind: v.Claim.Kind, Label: v.Label, Probabilities: v.Probabilities}
		for _, e := range ev {
			it.Sources = append(it.Sources, src{URL: e.Source, Score: e.Score, Extract: e.Snippet})
		}
		items[i] = it
	}
	in, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return textCall(ctx, s.client, s.model, summarizeSystemPrompt, "<claims>\n"+string(in)+"\n</claims>")
}
