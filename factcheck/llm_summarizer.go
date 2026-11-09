package factcheck

import (
	"context"
	_ "embed"
	"encoding/json"
	"sort"
)

//go:embed prompts/summarize.system.md
var summarizeSystemPrompt string

// LLMSummarizer drafts the final report. Labels and probabilities are
// computed upstream; the model only writes them up.
type LLMSummarizer struct {
	llm LLM
}

func NewLLMSummarizer(llm LLM) *LLMSummarizer { return &LLMSummarizer{llm: llm} }

func (s *LLMSummarizer) Summarize(ctx context.Context, verdicts []Verdict) (string, error) {
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
	return s.llm.Text(ctx, summarizeSystemPrompt, "<claims>\n"+string(in)+"\n</claims>")
}
