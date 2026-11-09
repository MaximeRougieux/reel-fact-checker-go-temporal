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

func ExtractorModel() string { return get("ANTHROPIC_MODEL", "claude-haiku-5-5") }

func Fetcher() string  { return get("FETCHER", "mock") } // "mock" or "browser"
func AudioDir() string { return get("AUDIO_DIR", "data/audio") }
func FFmpeg() string   { return get("FFMPEG_PATH", "ffmpeg") }
