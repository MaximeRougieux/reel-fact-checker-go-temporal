package factcheck

import (
	"context"
	"sort"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

// Activities holds the clients injected at Worker startup.
type Activities struct {
	Fetcher     AudioFetcher
	Transcriber Transcriber
	Extractor   ClaimExtractor
	Searcher    Searcher
	Judge       ClaimJudge
	Summarizer  Summarizer
}

// ValidateReelURL returns the canonical URL. An invalid URL can never succeed,
// so the error is non-retryable.
func (a *Activities) ValidateReelURL(_ context.Context, raw string) (string, error) {
	canonical, err := ParseReelURL(raw)
	if err != nil {
		return "", temporal.NewNonRetryableApplicationError(err.Error(), "InvalidReelURL", err)
	}
	return canonical, nil
}

func (a *Activities) FetchAudio(ctx context.Context, reelURL string) (string, error) {
	activity.RecordHeartbeat(ctx, "fetching")
	return a.Fetcher.Fetch(ctx, reelURL)
}

func (a *Activities) Transcribe(ctx context.Context, audioRef string) (string, error) {
	activity.RecordHeartbeat(ctx, "transcribing")
	return a.Transcriber.Transcribe(ctx, audioRef)
}

func (a *Activities) ExtractClaims(ctx context.Context, transcript string) ([]Claim, error) {
	return a.Extractor.Extract(ctx, transcript)
}

// CheckClaim retrieves evidence, then judges the claim against it. With no
// evidence the verdict is insufficient_evidence, not an error.
func (a *Activities) CheckClaim(ctx context.Context, claim Claim) (Verdict, error) {
	ev, err := a.Searcher.Search(ctx, claim)
	if err != nil {
		return Verdict{}, err
	}
	v := Verdict{Claim: claim, Evidence: ev}
	if len(ev) == 0 {
		v.Label, v.Confidence = Insufficient, 1
		return v, nil
	}
	probs, err := a.Judge.Judge(ctx, claim, ev)
	if err != nil {
		return Verdict{}, err
	}
	v.Probabilities = probs
	labels := make([]string, 0, len(probs))
	for l := range probs {
		labels = append(labels, l)
	}
	sort.Strings(labels) // deterministic tie-break
	for _, l := range labels {
		if probs[l] > v.Confidence {
			v.Label, v.Confidence = l, probs[l]
		}
	}
	if v.Label != Insufficient && v.Confidence < ContestedBelow {
		v.Label = Contested
	}
	return v, nil
}

func (a *Activities) SummarizeReport(ctx context.Context, verdicts []Verdict) (string, error) {
	return a.Summarizer.Summarize(ctx, verdicts)
}
