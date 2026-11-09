package factcheck

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

// resolveAudioFile maps a file:// audio reference to an absolute path, and
// only accepts files under dir so a reference cannot point at arbitrary files.
func resolveAudioFile(audioRef, dir string) (string, error) {
	path, ok := strings.CutPrefix(audioRef, "file://")
	if !ok {
		return "", temporal.NewNonRetryableApplicationError("unsupported audio reference", "BadAudioRef", nil)
	}
	root, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil || !strings.HasPrefix(abs, root+string(filepath.Separator)) {
		return "", temporal.NewNonRetryableApplicationError("audio reference outside the audio directory", "BadAudioRef", nil)
	}
	return abs, nil
}

// heartbeatEvery records an Activity heartbeat on a timer until the returned
// stop function is called, so a crashed Worker is noticed quickly.
func heartbeatEvery(ctx context.Context, every time.Duration, detail string) (stop func()) {
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				activity.RecordHeartbeat(ctx, detail)
			}
		}
	}()
	return func() { close(done) }
}
