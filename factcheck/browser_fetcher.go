package factcheck

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

const maxMediaBytes = 100 << 20

// BrowserFetcher loads one validated reel URL in a headless Chrome, as an
// ordinary visitor, records the media URLs the page requests, downloads them
// and extracts mono 16 kHz audio with ffmpeg. Audio is written to Dir and the
// returned reference is a file path, so the Worker must be able to read it
// from the Transcribe Activity (same host, or swap Dir for shared storage).
//
// No stealth or evasion: if Instagram shows a login wall or serves no media,
// Fetch returns a non-retryable ReelNotRetrievable error.
type BrowserFetcher struct {
	Dir     string
	FFmpeg  string
	Timeout time.Duration
}

func notRetrievable(format string, args ...any) error {
	return temporal.NewNonRetryableApplicationError(fmt.Sprintf(format, args...), "ReelNotRetrievable", nil)
}

func (b *BrowserFetcher) Fetch(ctx context.Context, reelURL string) (string, error) {
	canonical, err := ParseReelURL(reelURL) // defence in depth: never browse an unvalidated URL
	if err != nil {
		return "", temporal.NewNonRetryableApplicationError(err.Error(), "InvalidReelURL", err)
	}
	if err := os.MkdirAll(b.Dir, 0o755); err != nil {
		return "", err
	}

	mediaURLs, err := b.collectMediaURLs(ctx, canonical)
	if err != nil {
		return "", err
	}
	if len(mediaURLs) == 0 {
		return "", notRetrievable("no media found on the reel page (private, removed or blocked)")
	}

	code := shortcode(canonical)
	for i, mu := range mediaURLs {
		activity.RecordHeartbeat(ctx, fmt.Sprintf("downloading %d/%d", i+1, len(mediaURLs)))
		raw := filepath.Join(b.Dir, fmt.Sprintf("%s.%d.mp4", code, i))
		if err := download(ctx, mu, raw); err != nil {
			continue
		}
		out := filepath.Join(b.Dir, code+".wav")
		cmd := exec.CommandContext(ctx, b.ffmpeg(), "-y", "-i", raw, "-map", "0:a:0", "-vn", "-ac", "1", "-ar", "16000", out)
		runErr := cmd.Run()
		os.Remove(raw)
		if runErr == nil {
			return "file://" + out, nil // first track that has an audio stream
		}
	}
	return "", notRetrievable("media was found but none of it contained audio (is ffmpeg working?)")
}

func (b *BrowserFetcher) ffmpeg() string {
	if b.FFmpeg != "" {
		return b.FFmpeg
	}
	return "ffmpeg"
}

func (b *BrowserFetcher) collectMediaURLs(ctx context.Context, reelURL string) ([]string, error) {
	timeout := b.Timeout
	if timeout == 0 {
		timeout = 45 * time.Second
	}
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, chromedp.DefaultExecAllocatorOptions[:]...)
	defer cancelAlloc()
	taskCtx, cancelTask := chromedp.NewContext(allocCtx)
	defer cancelTask()
	taskCtx, cancelTimeout := context.WithTimeout(taskCtx, timeout)
	defer cancelTimeout()

	var mu sync.Mutex
	seen := map[string]bool{}
	var urls []string
	chromedp.ListenTarget(taskCtx, func(ev any) {
		e, ok := ev.(*network.EventResponseReceived)
		if !ok {
			return
		}
		mt := strings.ToLower(e.Response.MimeType)
		if !strings.HasPrefix(mt, "video/") && !strings.HasPrefix(mt, "audio/") {
			return
		}
		clean, ok := cleanMediaURL(e.Response.URL)
		if !ok {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if !seen[clean] {
			seen[clean] = true
			urls = append(urls, clean)
		}
	})

	var location string
	if err := chromedp.Run(taskCtx, network.Enable(), chromedp.Navigate(reelURL)); err != nil {
		return nil, fmt.Errorf("browser navigation failed: %w", err) // retryable: may be transient
	}
	// Give the player a few seconds to request its media.
	for i := 0; i < 20; i++ {
		mu.Lock()
		n := len(urls)
		mu.Unlock()
		if n > 0 && i >= 4 {
			break
		}
		if err := chromedp.Run(taskCtx, chromedp.Sleep(time.Second)); err != nil {
			break
		}
	}
	_ = chromedp.Run(taskCtx, chromedp.Location(&location))
	if loc, err := url.Parse(location); err == nil {
		if !allowedHosts[strings.ToLower(loc.Hostname())] {
			return nil, notRetrievable("page redirected away from Instagram")
		}
		if strings.Contains(loc.Path, "/accounts/login") {
			return nil, notRetrievable("Instagram requires login for this reel")
		}
	}
	mu.Lock()
	defer mu.Unlock()
	return append([]string(nil), urls...), nil
}

// cleanMediaURL keeps only Instagram CDN hosts over https and strips the byte
// range parameters so the whole track can be downloaded in one request.
func cleanMediaURL(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" {
		return "", false
	}
	h := strings.ToLower(u.Hostname())
	if !strings.HasSuffix(h, ".fbcdn.net") && !strings.HasSuffix(h, ".cdninstagram.com") {
		return "", false
	}
	q := u.Query()
	q.Del("bytestart")
	q.Del("byteend")
	u.RawQuery = q.Encode()
	return u.String(), true
}

func download(ctx context.Context, mediaURL, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("media download: %s", resp.Status)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	n, err := io.Copy(f, io.LimitReader(resp.Body, maxMediaBytes+1))
	if err == nil && n > maxMediaBytes {
		err = fmt.Errorf("media larger than %d bytes", maxMediaBytes)
	}
	return err
}
