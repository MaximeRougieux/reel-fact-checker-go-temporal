package factcheck

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"
)

const MaxClaims = 20

func withTimeout(ctx workflow.Context, d time.Duration, heartbeat time.Duration) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: d,
		HeartbeatTimeout:    heartbeat,
	})
}

// FactCheckWorkflow validates the reel URL before anything touches the network,
// then transcribes, extracts claims, checks each claim in parallel and summarizes.
func FactCheckWorkflow(ctx workflow.Context, reelURL string) (Report, error) {
	var a *Activities

	stage := "validating the link"
	if err := workflow.SetQueryHandler(ctx, "stage", func() (string, error) { return stage, nil }); err != nil {
		return Report{}, err
	}

	var canonical string
	if err := workflow.ExecuteActivity(withTimeout(ctx, 5*time.Second, 0), a.ValidateReelURL, reelURL).Get(ctx, &canonical); err != nil {
		return Report{}, err
	}

	stage = "fetching the reel audio"
	var audioRef string
	if err := workflow.ExecuteActivity(withTimeout(ctx, 120*time.Second, 30*time.Second), a.FetchAudio, canonical).Get(ctx, &audioRef); err != nil {
		return Report{}, err
	}

	stage = "transcribing"
	var transcript string
	if err := workflow.ExecuteActivity(withTimeout(ctx, 300*time.Second, 60*time.Second), a.Transcribe, audioRef).Get(ctx, &transcript); err != nil {
		return Report{}, err
	}

	stage = "extracting claims"
	var claims []Claim
	if err := workflow.ExecuteActivity(withTimeout(ctx, 60*time.Second, 0), a.ExtractClaims, transcript).Get(ctx, &claims); err != nil {
		return Report{}, err
	}
	if len(claims) > MaxClaims {
		claims = claims[:MaxClaims] // extractor returns them ordered by check-worthiness
	}

	stage = fmt.Sprintf("checking %d claims against sources", len(claims))
	checkCtx := withTimeout(ctx, 120*time.Second, 0)
	futures := make([]workflow.Future, len(claims))
	verdicts := make([]Verdict, len(claims))
	for i, c := range claims {
		if !c.Verifiable {
			verdicts[i] = Verdict{Claim: c, Label: NotCheckable} // no search spent on private matters
			continue
		}
		futures[i] = workflow.ExecuteActivity(checkCtx, a.CheckClaim, c)
	}
	for i, f := range futures {
		if f == nil {
			continue
		}
		if err := f.Get(ctx, &verdicts[i]); err != nil {
			return Report{}, err
		}
	}

	stage = "writing the report"
	var summary string
	if err := workflow.ExecuteActivity(withTimeout(ctx, 60*time.Second, 0), a.SummarizeReport, verdicts).Get(ctx, &summary); err != nil {
		return Report{}, err
	}
	return Report{ReelURL: canonical, Verdicts: verdicts, Summary: summary}, nil
}
