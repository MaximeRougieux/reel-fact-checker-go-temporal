package factcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"

	"reel-fact-checker-go-temporal/internal/httperr"
	"reel-fact-checker-go-temporal/internal/llmconfig/mistral"
)

// VoxtralTranscriber sends the audio file to Mistral's Voxtral transcription
// API. The key is read from MISTRAL_API_KEY and sent as a bearer token, never
// logged. The language is left unset so the model detects it.
type VoxtralTranscriber struct {
	BaseURL  string
	Model    string
	AudioDir string // only files under this directory may be uploaded
	http     *http.Client
}

func NewVoxtralTranscriber(model, audioDir string) *VoxtralTranscriber {
	return &VoxtralTranscriber{BaseURL: mistral.DefaultBaseURL, Model: model, AudioDir: audioDir,
		http: &http.Client{Timeout: 240 * time.Second}}
}

func (v *VoxtralTranscriber) Transcribe(ctx context.Context, audioRef string) (string, error) {
	path, err := resolveAudioFile(audioRef, v.AudioDir)
	if err != nil {
		return "", err
	}
	audio, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	if err := form.WriteField("model", v.Model); err != nil {
		return "", err
	}
	part, err := form.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return "", err
	}
	if _, err := part.Write(audio); err != nil {
		return "", err
	}
	if err := form.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.BaseURL+"/v1/audio/transcriptions", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+os.Getenv("MISTRAL_API_KEY"))

	defer heartbeatEvery(ctx, 10*time.Second, "transcribing")()
	resp, err := v.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if err := httperr.StatusError("Voxtral", resp); err != nil {
		return "", err
	}

	var out struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("voxtral: bad response: %w", err)
	}
	transcript := strings.Join(strings.Fields(out.Text), " ")
	if transcript == "" {
		return "", temporal.NewNonRetryableApplicationError("no speech detected", "NoSpeech", nil)
	}
	return transcript, nil
}
