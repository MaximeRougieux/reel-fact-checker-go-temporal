package factcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const mistralBaseURL = "https://api.mistral.ai"

// MistralLLM is the Mistral provider, over plain HTTP. The key is read from
// MISTRAL_API_KEY and sent as a bearer token, never logged. There are no
// client-side retries: Temporal owns retry.
type MistralLLM struct {
	BaseURL string
	Model   string
	http    *http.Client
}

func NewMistralLLM(model string) *MistralLLM {
	return &MistralLLM{BaseURL: mistralBaseURL, Model: model, http: &http.Client{Timeout: 120 * time.Second}}
}

func (m *MistralLLM) Structured(ctx context.Context, system string, schema map[string]any, user string) (string, error) {
	return m.chat(ctx, map[string]any{
		"model":       m.Model,
		"max_tokens":  4096,
		"temperature": 0,
		"messages":    messages(system, user),
		"response_format": map[string]any{
			"type":        "json_schema",
			"json_schema": map[string]any{"name": "result", "schema": schema, "strict": true},
		},
	})
}

func (m *MistralLLM) Text(ctx context.Context, system, user string) (string, error) {
	return m.chat(ctx, map[string]any{
		"model":      m.Model,
		"max_tokens": 8192,
		"messages":   messages(system, user),
	})
}

func messages(system, user string) []map[string]string {
	return []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}}
}

func (m *MistralLLM) chat(ctx context.Context, payload map[string]any) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.BaseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+os.Getenv("MISTRAL_API_KEY"))

	resp, err := m.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if err := statusError("Mistral", resp); err != nil {
		return "", err
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("mistral: bad response: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("mistral: no choices in response")
	}
	if r := out.Choices[0].FinishReason; r == "length" || r == "model_length" {
		return "", fmt.Errorf("model stopped: %s", r)
	}
	return messageText(out.Choices[0].Message.Content)
}

// messageText reads message content, which is a string or a list of chunks.
func messageText(raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	var chunks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &chunks); err != nil {
		return "", fmt.Errorf("mistral: unexpected message content")
	}
	var b strings.Builder
	for _, c := range chunks {
		if c.Type == "text" {
			b.WriteString(c.Text)
		}
	}
	return b.String(), nil
}
