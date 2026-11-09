package factcheck

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed prompts/judge.system.md
var judgeSystemPrompt string

//go:embed prompts/judge.schema.json
var judgeSchemaJSON []byte

// LLMJudge asks the model for a stance per evidence snippet, then computes
// the verdict probabilities in code from those stances weighted by the search
// provider's relevance score. Model-reported probabilities are poorly
// calibrated, so confidence comes from how much of the evidence agrees.
type LLMJudge struct {
	llm    LLM
	schema map[string]any
}

func NewLLMJudge(llm LLM) (*LLMJudge, error) {
	var schema map[string]any
	if err := json.Unmarshal(judgeSchemaJSON, &schema); err != nil {
		return nil, fmt.Errorf("bad judge schema: %w", err)
	}
	return &LLMJudge{llm: llm, schema: schema}, nil
}

// insufficientWeight is how much decisive evidence it takes to halve the
// "insufficient" share. Neutral snippets are uninformative and never count as
// votes: they must not outweigh one article that settles the claim.
const insufficientWeight = 0.5

// partialWeight is how much a partial stance counts compared with a full one.
const partialWeight = 0.5

func (j *LLMJudge) Judge(ctx context.Context, claim Claim, evidence []Evidence) (map[string]float64, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "<claim>%s</claim>\n", claim.Text)
	if len(claim.Premises) > 0 {
		fmt.Fprintf(&b, "<premises>%s</premises>\n", strings.Join(claim.Premises, " | "))
	}
	for i, e := range evidence {
		fmt.Fprintf(&b, "<snippet index=\"%d\" source=\"%s\">%s</snippet>\n", i, e.Source, e.Snippet)
	}
	text, err := j.llm.Structured(ctx, judgeSystemPrompt, j.schema, b.String())
	if err != nil {
		return nil, err
	}
	var out struct {
		Stances []struct {
			Index  int    `json:"index"`
			Stance string `json:"stance"`
		} `json:"stances"`
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return nil, fmt.Errorf("invalid judge output: %w", err)
	}
	var sup, ref float64
	for _, st := range out.Stances {
		if st.Index < 0 || st.Index >= len(evidence) {
			continue
		}
		w := evidence[st.Index].Score
		if w <= 0 {
			w = 0.5
		}
		switch st.Stance {
		case "supports":
			sup += w
		case "partially_supports":
			sup += partialWeight * w
		case "refutes":
			ref += w
		case "partially_refutes":
			ref += partialWeight * w
		}
	}
	decisive := sup + ref
	if decisive == 0 {
		return map[string]float64{Supported: 0, Refuted: 0, Insufficient: 1}, nil
	}
	insufficient := insufficientWeight / (insufficientWeight + decisive)
	rest := 1 - insufficient
	return map[string]float64{
		Supported:    rest * sup / decisive,
		Refuted:      rest * ref / decisive,
		Insufficient: insufficient,
	}, nil
}
