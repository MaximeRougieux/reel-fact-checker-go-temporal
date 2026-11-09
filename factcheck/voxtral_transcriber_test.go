package factcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"go.temporal.io/sdk/temporal"
)

func voxtralAgainst(t *testing.T, h http.HandlerFunc) (*VoxtralTranscriber, string) {
	t.Helper()
	t.Setenv("MISTRAL_API_KEY", "test-key")
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	v := NewVoxtralTranscriber("voxtral-mini-latest", dir)
	v.BaseURL = srv.URL
	file := filepath.Join(dir, "abc.wav")
	if err := os.WriteFile(file, []byte("RIFF-fake-audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	return v, "file://" + file
}

func TestVoxtralUploadsFileAndReturnsText(t *testing.T) {
	v, ref := voxtralAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/transcriptions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("bad request: %s auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("model") != "voxtral-mini-latest" {
			t.Errorf("model = %q", r.FormValue("model"))
		}
		f, hdr, err := r.FormFile("file")
		if err != nil || hdr.Filename != "abc.wav" {
			t.Fatalf("file part: %v %v", hdr, err)
		}
		f.Close()
		w.Write([]byte(`{"text":"  bonjour   tout le monde "}`))
	})
	text, err := v.Transcribe(context.Background(), ref)
	if err != nil || text != "bonjour tout le monde" {
		t.Fatalf("got %q, %v", text, err)
	}
}

func TestVoxtralRejectsFilesOutsideAudioDir(t *testing.T) {
	v, _ := voxtralAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("must not call the API")
	})
	for _, ref := range []string{"file:///etc/passwd", "https://example.com/a.wav", "mock://audio/x"} {
		_, err := v.Transcribe(context.Background(), ref)
		ae, ok := err.(*temporal.ApplicationError)
		if !ok || !ae.NonRetryable() {
			t.Errorf("%s: want non-retryable error, got %v", ref, err)
		}
	}
}

func TestVoxtralEmptyTranscriptIsNoSpeech(t *testing.T) {
	v, ref := voxtralAgainst(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"text":"  "}`)) })
	_, err := v.Transcribe(context.Background(), ref)
	if ae, ok := err.(*temporal.ApplicationError); !ok || ae.Type() != "NoSpeech" {
		t.Fatalf("got %v", err)
	}
}
