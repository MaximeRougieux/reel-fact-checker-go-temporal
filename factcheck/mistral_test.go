package factcheck

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.temporal.io/sdk/temporal"
)

func mistralAgainst(t *testing.T, h http.HandlerFunc) *MistralLLM {
	t.Helper()
	t.Setenv("MISTRAL_API_KEY", "test-key")
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	m := NewMistralLLM("mistral-small-latest")
	m.BaseURL = srv.URL
	return m
}

func TestMistralStructuredRequestAndReply(t *testing.T) {
	var got map[string]any
	m := mistralAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("bad request: %s auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"choices":[{"message":{"content":"{\"ok\":true}"},"finish_reason":"stop"}]}`))
	})
	schema := map[string]any{"type": "object"}
	text, err := m.Structured(context.Background(), "sys", schema, "usr")
	if err != nil || text != `{"ok":true}` {
		t.Fatalf("got %q, %v", text, err)
	}
	rf := got["response_format"].(map[string]any)
	js := rf["json_schema"].(map[string]any)
	if rf["type"] != "json_schema" || js["strict"] != true || js["schema"] == nil {
		t.Errorf("response_format = %v", rf)
	}
	msgs := got["messages"].([]any)
	if len(msgs) != 2 || msgs[0].(map[string]any)["role"] != "system" || msgs[1].(map[string]any)["content"] != "usr" {
		t.Errorf("messages = %v", msgs)
	}
}

func TestMistralChunkedContent(t *testing.T) {
	m := mistralAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"content":[{"type":"text","text":"he"},{"type":"text","text":"llo"}]},"finish_reason":"stop"}]}`))
	})
	if text, err := m.Text(context.Background(), "s", "u"); err != nil || text != "hello" {
		t.Fatalf("got %q, %v", text, err)
	}
}

func TestMistralTruncatedIsAnError(t *testing.T) {
	m := mistralAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"content":"{\"cl"},"finish_reason":"length"}]}`))
	})
	if _, err := m.Text(context.Background(), "s", "u"); err == nil {
		t.Fatal("want error on finish_reason=length")
	}
}

func TestProviderStatusErrors(t *testing.T) {
	cases := []struct {
		status       int
		retryAfter   string
		nonRetryable bool
		delay        time.Duration
	}{
		{http.StatusUnauthorized, "", true, 0},
		{http.StatusUnprocessableEntity, "", true, 0},
		{http.StatusTooManyRequests, "7", false, 7 * time.Second},
		{http.StatusInternalServerError, "", false, 0},
	}
	for _, c := range cases {
		m := mistralAgainst(t, func(w http.ResponseWriter, r *http.Request) {
			if c.retryAfter != "" {
				w.Header().Set("Retry-After", c.retryAfter)
			}
			w.WriteHeader(c.status)
		})
		_, err := m.Text(context.Background(), "s", "u")
		var appErr *temporal.ApplicationError
		isApp := asApplicationError(err, &appErr)
		if c.status == http.StatusInternalServerError {
			if err == nil || isApp {
				t.Errorf("500: want plain retryable error, got %v", err)
			}
			continue
		}
		if !isApp || appErr.NonRetryable() != c.nonRetryable || appErr.NextRetryDelay() != c.delay {
			t.Errorf("status %d: got %v", c.status, err)
		}
	}
}

func asApplicationError(err error, target **temporal.ApplicationError) bool {
	ae, ok := err.(*temporal.ApplicationError)
	if ok {
		*target = ae
	}
	return ok
}
