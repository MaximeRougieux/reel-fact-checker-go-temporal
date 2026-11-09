# Design: Reel fact-checker

**What this builds:** Given an Instagram reel URL: validate it, get the audio, transcribe it, extract checkable claims (facts and implications), check each claim against news sources in parallel, and write a report of claims, verdicts, probabilities and sources.

## Workflow

`FactCheckWorkflow(reelURL string) -> Report{ReelURL, Verdicts, Summary}`

validate -> fetch audio -> transcribe -> extract claims -> check each claim in parallel (capped at 20) -> summarize.

## Activities

| Activity | Arguments | Timeout | Retry | Real or mock |
|---|---|---|---|---|
| `ValidateReelURL` | `url` | 5s | non-retryable if invalid | real (pure code, allowlist) |
| `FetchAudio` | canonical url | 120s, heartbeat 30s | non-retryable `ReelNotRetrievable` (login wall, no media, redirect off Instagram); default otherwise | headless Chrome + ffmpeg (`FETCHER=browser`), mock by default |
| `Transcribe` | `audioRef` | 300s, heartbeat 60s | default; non-retryable on bad ref / no speech | local whisper.cpp, Turbo model, language auto-detect (`TRANSCRIBER=whisper`), or Mistral Voxtral (`TRANSCRIBER=voxtral`, `VOXTRAL_MODEL`); mock by default |
| `ExtractClaims` | `transcript` | 60s | default | LLM provider (Claude or Mistral), structured output |
| `CheckClaim` x N | `claim` | 120s | default; Tavily 429 retries after its Retry-After; 401 non-retryable | Tavily search, then LLM judge |
| `SummarizeReport` | `verdicts` | 60s | default | LLM provider |

Search: Tavily `basic` depth by default (`TAVILY_DEPTH=advanced` for longer extracts, 2 credits), `topic=news` (`TAVILY_TOPIC`), social platforms excluded, results scored below 0.3 dropped. Identical queries are served from an in-memory cache (24h, per Worker) at no credit cost.

Claims the extractor marks `verifiable: false` (private matters no public source could settle) are not searched and appear as `not_checkable`.

Judge: the model labels each snippet supports / refutes / neutral. Probabilities are computed in code from the relevance-weighted stances. Neutral snippets are not votes: "insufficient" shrinks as decisive evidence accumulates, and is 1.0 with none. A supported/refuted verdict whose top probability is under 0.6 is reported as `contested`.

Text models sit behind one `LLM` interface with two providers: Claude (`ANTHROPIC_API_KEY`, `ANTHROPIC_MODEL`, default `claude-haiku-5-5`) and Mistral (`MISTRAL_API_KEY`, `MISTRAL_MODEL`, default `mistral-small-latest`, JSON-schema structured output). `LLM_PROVIDER` picks one; otherwise the provider whose key is set is used. The extractor, judge and summarizer take any `LLM`, and `MISTRAL_JUDGE_MODEL` / `ANTHROPIC_JUDGE_MODEL` can set a different model for judging and summarizing.

All clients sit behind interfaces and are built once per Worker (dependency injection). Prompts and schemas are in `factcheck/prompts/`. Models can be swapped per step by env var; a Jev-style decision model would slot in behind `ClaimJudge`.

## Entry points and caching

- `cmd/server`: `POST /api/check {url, refresh}` validates the link, then starts or reuses a run; `GET /api/check/{id}` returns the stage while running and the report when done; `GET /` is the page. Listens on 127.0.0.1:8080 (`SERVER_ADDR`), no auth, because it triggers paid calls.
- Cache: the Workflow ID is `factcheck-<reel shortcode>`. A completed run for the same reel is returned instead of recomputed (`refresh: true` runs it again under the same ID). Concurrent requests for one reel share a run (`USE_EXISTING`). The cache lasts as long as Temporal keeps the closed run (namespace retention); the local dev server is in memory unless started with `--db-filename`.

## State and messages

- State: claims and verdicts (local variables). Audio is a local file reference, so Worker and transcriber share a host.
- Queries: `stage` (what the run is doing, shown in the UI). Signals / Updates: none.

## Failure

Invalid URL or an unretrievable reel fails the Workflow (non-retryable). A claim with no evidence is "insufficient evidence", not an error. More than 20 claims: the first 20 (ordered by importance) are kept.

## Patterns

| Pattern | Why it applies here |
|---|---|
| `parallel-execution` | Claim checks are independent, at most 20 pending |
| `activity-dependency-injection` | Browser, transcriber, LLM and Tavily clients built once per Worker |
| `non-retryable-errors` | Invalid URL, login wall and bad key can never succeed |
| `delayed-retry` | Tavily returns 429 with Retry-After |
| `long-running-activity` | Fetch and transcribe run longer than 30s, so they heartbeat |

## Verify

Last run (reel De1eZNCodUl): Completed in ~43s with six claims: one supported (91%), one refuted (77%), two contested, two insufficient evidence.

## Not yet

- Source quality beyond "news only" (no allowlist of reputable outlets).
- The judge checks the gist of a claim, not details such as exact times.
- Extraction is not deterministic: the same reel can yield different claim sets.
- The judge now has partial stances (half weight), but the UI lists the top sources by relevance even when the judge found them neutral.
- No rate limiting on Tavily; Jev not tried; audio storage is local disk.
- Fetching Instagram with a headless browser is fragile and may break or be blocked.

## Changed this round

Judge: added `partially_supports` / `partially_refutes` (half weight; details like times and numbers must match for a full stance). Added whole-run caching by reel id, the `stage` Query, and a small API and web page.
