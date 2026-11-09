package factcheck

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
)

// WhisperTranscriber runs whisper.cpp locally. Language is auto-detected
// (whisper-cli defaults to English, which garbles other languages) and context
// carry-over is off to avoid repetition loops on long audio.
type WhisperTranscriber struct {
	CLI      string // whisper-cli binary
	Model    string // ggml model path
	AudioDir string // only files under this directory may be transcribed
}

func (w *WhisperTranscriber) Transcribe(ctx context.Context, audioRef string) (string, error) {
	abs, err := resolveAudioFile(audioRef, w.AudioDir)
	if err != nil {
		return "", err
	}
	defer heartbeatEvery(ctx, 10*time.Second, "transcribing")()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, w.CLI, "-m", w.Model, "-f", abs, "-l", "auto", "-mc", "0", "-nt", "-np")
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("whisper failed: %w: %s", err, tail(stderr.String(), 300))
	}
	transcript := strings.Join(strings.Fields(stdout.String()), " ")
	if transcript == "" {
		return "", temporal.NewNonRetryableApplicationError("no speech detected", "NoSpeech", nil)
	}
	return transcript, nil
}

func tail(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}
