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

func Fetcher() string  { return Get("FETCHER", "mock") } // "mock" or "browser"
func AudioDir() string { return Get("AUDIO_DIR", "data/audio") }
func FFmpeg() string   { return Get("FFMPEG_PATH", "ffmpeg") }

func Transcriber() string  { return Get("TRANSCRIBER", "mock") } // "mock", "whisper" or "voxtral"
func WhisperCLI() string   { return Get("WHISPER_CLI", "whisper-cli") }
func WhisperModel() string { return Get("WHISPER_MODEL", "models/ggml-large-v3-turbo.bin") }

func VoxtralModel() string { return Get("VOXTRAL_MODEL", "voxtral-mini-latest") }

func SearchTopic() string { return Get("TAVILY_TOPIC", "news") } // "news" or "general"

func SearchDepth() string { return Get("TAVILY_DEPTH", "basic") } // "basic" or "advanced"
