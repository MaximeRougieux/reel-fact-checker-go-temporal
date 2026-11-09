# Design: Reel fact-checker

**What this builds:** Given an Instagram reel URL: validate it, get the audio, transcribe it, extract checkable claims (facts and implications), check each claim in parallel, and write a report of claims, verdicts, probabilities and sources.

## Workflow

`FactCheckWorkflow(reelURL string) -> Report{ReelURL, Verdicts, Summary}`

validate -> fetch audio -> transcribe -> extract claims -> check each claim in parallel (capped at 20) -> summarize.

## Activities

| Activity | Arguments | Timeout | Retry | Real or mock |
|---|---|---|---|---|
| `ValidateReelURL` | `url` | 5s | non-retryable if invalid | real (pure code, allowlist) |
| `FetchAudio` | canonical url | 120s, heartbeat 30s | non-retryable `ReelNotRetrievable`; default otherwise | mock |
| `Transcribe` | `audioRef` | 300s, heartbeat 60s | default; non-retryable on bad ref / no speech | mock |
| `ExtractClaims` | `transcript` | 60s | default | mock |
| `CheckClaim` x N | `claim` | 120s | default | mock search, then mock judge |
| `SummarizeReport` | `verdicts` | 60s | default | mock |

All clients sit behind interfaces and are built once per Worker (dependency injection), so each mock can be replaced by a real implementation without touching the Workflow.

## State and messages

- State: claims and verdicts (local variables).
- Queries: `stage` (what the run is doing). Signals / Updates: none.

## Failure

Invalid URL or an unretrievable reel fails the Workflow (non-retryable). A claim with no evidence is "insufficient evidence", not an error. More than 20 claims: the first 20 are kept.

## Patterns

| Pattern | Why it applies here |
|---|---|
| `parallel-execution` | Claim checks are independent, at most 20 pending |
| `activity-dependency-injection` | Clients built once per Worker |
| `non-retryable-errors` | Invalid URL and unretrievable reels can never succeed |
| `long-running-activity` | Fetch and transcribe run longer than 30s, so they heartbeat |
