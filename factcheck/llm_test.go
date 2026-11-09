package factcheck

import (
	"context"
	"math"
	"testing"
)

// fakeLLM answers every structured request with a fixed JSON document.
type fakeLLM struct{ structured string }

func (f fakeLLM) Structured(context.Context, string, map[string]any, string) (string, error) {
	return f.structured, nil
}
func (f fakeLLM) Text(context.Context, string, string) (string, error) { return "", nil }

func TestLLMJudgeProbabilities(t *testing.T) {
	ev := []Evidence{{Source: "a", Score: 0.8}, {Source: "b", Score: 0.4}, {Source: "c", Score: 0.9}}
	cases := []struct {
		name   string
		reply  string
		wantS  float64
		wantR  float64
		wantIn float64
	}{
		{"no stance is decisive", `{"stances":[{"index":0,"stance":"neutral"},{"index":2,"stance":"neutral"}]}`, 0, 0, 1},
		{"one supporting article", `{"stances":[{"index":0,"stance":"supports"},{"index":1,"stance":"neutral"}]}`, 0.8 / 1.3, 0, 0.5 / 1.3},
		{"partial counts half", `{"stances":[{"index":0,"stance":"partially_refutes"}]}`, 0, 0.4 / 0.9, 0.5 / 0.9},
		{"out of range index ignored", `{"stances":[{"index":7,"stance":"supports"}]}`, 0, 0, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			j, err := NewLLMJudge(fakeLLM{structured: c.reply})
			if err != nil {
				t.Fatal(err)
			}
			p, err := j.Judge(context.Background(), Claim{Text: "x"}, ev)
			if err != nil {
				t.Fatal(err)
			}
			for k, want := range map[string]float64{Supported: c.wantS, Refuted: c.wantR, Insufficient: c.wantIn} {
				if math.Abs(p[k]-want) > 1e-9 {
					t.Errorf("%s = %v, want %v", k, p[k], want)
				}
			}
		})
	}
}
