![Tests](https://github.com/MaximeRougieux/reel-fact-checker-go-temporal/actions/workflows/test.yml/badge.svg)
# reel-fact-checker-go-temporal

A durable fact-checking pipeline for Instagram reels, built on Temporal (Go) with pluggable LLM and speech providers (Mistral, Anthropic, whisper.cpp). It fetches the audio, transcribes it, extracts the claims and checks each one against news sources.

## Prerequisites

- Go 1.26+
- [Temporal CLI](https://docs.temporal.io/cli) (for the local dev server)

Only needed for real (non-mock) components:

- Google Chrome, and `ffmpeg` (audio fetch)
- Transcription, either local (`whisper-cpp` and a Whisper model) or through the Mistral API (Voxtral, no local install)
- A key for one LLM provider, [Anthropic](https://console.anthropic.com/) or [Mistral](https://console.mistral.ai/) (claim extraction, judging, summary), and a [Tavily API key](https://tavily.com/) (search)

## Setup

### 1. Temporal dev server

```sh
brew install temporal
temporal server start-dev --db-filename data/temporal.db
```

The server listens on `localhost:7233` and the Temporal UI is at http://localhost:8233. `--db-filename` keeps workflow history, and so the result cache, across restarts. Without it everything is in memory.

### 2. Configuration

```sh
cp .env.example .env
```

Fill in `TAVILY_API_KEY` and either `ANTHROPIC_API_KEY` or `MISTRAL_API_KEY` in `.env` (never commit it). The LLM provider is picked from whichever key is set; set `LLM_PROVIDER=claude` or `LLM_PROVIDER=mistral` to choose when both are. Without the keys, the Worker falls back to mock components and logs which implementation each step uses.

### 3. Real audio and transcription (optional)

Both options need `ffmpeg` and Chrome to fetch the audio.

- **Voxtral (Mistral API):** set `MISTRAL_API_KEY` and run the Worker with `TRANSCRIBER=voxtral`. Nothing else to install.
- **Local whisper.cpp:** install it and download a model, then run the Worker with `TRANSCRIBER=whisper`.

```sh
brew install ffmpeg whisper-cpp
mkdir -p models
curl -L -o models/ggml-large-v3-turbo.bin \
  https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo.bin
```

## Run

Use one terminal each, from the project root, after the Temporal server is running.

**Worker** (runs the Workflow and Activities):

```sh
# real fetcher and local transcriber (use TRANSCRIBER=voxtral for the Mistral API)
FETCHER=browser TRANSCRIBER=whisper go run ./cmd/worker

# or, all mocks
go run ./cmd/worker
```

**API and web page:**

```sh
go run ./cmd/server
```

Open http://127.0.0.1:8080, paste a public reel link (`https://www.instagram.com/reel/<code>/`) and press Check. A fresh reel takes about 45 seconds. Checking the same reel again returns the cached result, and "Re-run" forces a new run. Each run shows up in the Temporal UI at http://localhost:8233.

**CLI alternative:**

```sh
go run ./cmd/starter https://www.instagram.com/reel/<code>/
```

The server has no authentication and listens on localhost only. Each new reel spends Tavily credits and model calls.

## Tests

```sh
go test ./...
```

See [DESIGN.md](DESIGN.md) for the design and known gaps.
