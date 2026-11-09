package config

import (
	"bufio"
	"os"
	"strings"
)

// LoadDotEnv loads KEY=VALUE lines from path into the environment without
// overriding variables that are already set. A missing file is not an error.
func LoadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if os.Getenv(k) == "" {
			os.Setenv(k, strings.Trim(strings.TrimSpace(v), `"'`))
		}
	}
}

// LLMProvider picks the text-model provider: LLM_PROVIDER if set, otherwise
// whichever API key is present. Empty means no provider is configured and the
// Worker uses mocks.
func LLMProvider() string {
	if p := os.Getenv("LLM_PROVIDER"); p != "" {
		return strings.ToLower(p)
	}
	switch {
	case os.Getenv("ANTHROPIC_API_KEY") != "":
		return "claude"
	case os.Getenv("MISTRAL_API_KEY") != "":
		return "mistral"
	}
	return ""
}

// LLMKeyVar is the environment variable holding a provider's API key.
func LLMKeyVar(provider string) string {
	if provider == "mistral" {
		return "MISTRAL_API_KEY"
	}
	return "ANTHROPIC_API_KEY"
}

func ExtractorModel() string {
	if LLMProvider() == "mistral" {
		return get("MISTRAL_MODEL", "mistral-small-latest")
	}
	return get("ANTHROPIC_MODEL", "claude-haiku-5-5")
}

func Fetcher() string  { return get("FETCHER", "mock") } // "mock" or "browser"
func AudioDir() string { return get("AUDIO_DIR", "data/audio") }
func FFmpeg() string   { return get("FFMPEG_PATH", "ffmpeg") }

func Transcriber() string  { return get("TRANSCRIBER", "mock") } // "mock" or "whisper"
func WhisperCLI() string   { return get("WHISPER_CLI", "whisper-cli") }
func WhisperModel() string { return get("WHISPER_MODEL", "models/ggml-large-v3-turbo.bin") }

func JudgeModel() string {
	return get("ANTHROPIC_JUDGE_MODEL", ExtractorModel())
}

func SearchTopic() string { return get("TAVILY_TOPIC", "news") } // "news" or "general"

func SearchDepth() string { return get("TAVILY_DEPTH", "basic") } // "basic" or "advanced"
