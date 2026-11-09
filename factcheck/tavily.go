package factcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.temporal.io/sdk/temporal"
)

const tavilyEndpoint = "https://api.tavily.com/search"

// TavilySearcher retrieves web evidence for a claim. The key is read from
// TAVILY_API_KEY and sent as a bearer token, never logged.
type TavilySearcher struct {
	MaxResults int
	Topic      string  // "news" restricts results to news sources
	Depth      string  // "basic" (1 credit) or "advanced" (2 credits, longer extracts)
	MinScore   float64 // drop results the provider scores below this
	http       *http.Client

	mu    sync.Mutex
	cache map[string]cachedSearch // identical queries cost no extra credits
}

type cachedSearch struct {
	at time.Time
	ev []Evidence
}

const cacheTTL = 24 * time.Hour

// Social platforms give short, unreliable extracts, so they are excluded.
var excludedDomains = []string{
	"instagram.com", "facebook.com", "tiktok.com", "x.com", "twitter.com",
	"youtube.com", "reddit.com", "tavily.com",
}

func NewTavilySearcher(topic, depth string) *TavilySearcher {
	return &TavilySearcher{MaxResults: 6, Topic: topic, Depth: depth, MinScore: 0.3, cache: map[string]cachedSearch{}, http: &http.Client{Timeout: 60 * time.Second}}
}

func (t *TavilySearcher) Search(ctx context.Context, claim Claim) ([]Evidence, error) {
	query := claim.Text
	if len(claim.Premises) > 0 {
		query += " " + strings.Join(claim.Premises, " ")
	}
	key := t.Topic + "|" + t.Depth + "|" + query
	t.mu.Lock()
	if c, ok := t.cache[key]; ok && time.Since(c.at) < cacheTTL {
		t.mu.Unlock()
		return c.ev, nil
	}
	t.mu.Unlock()

	payload := map[string]any{
		"query":           query,
		"search_depth":    t.Depth,
		"max_results":     t.MaxResults,
		"topic":           t.Topic,
		"exclude_domains": excludedDomains,
		"include_answer":  false,
	}
	if t.Depth == "advanced" {
		payload["chunks_per_source"] = 3 // longer, query-relevant extracts
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tavilyEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+os.Getenv("TAVILY_API_KEY"))

	resp, err := t.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, temporal.NewNonRetryableApplicationError("Tavily rejected the API key", "SearchAuth", nil)
	case resp.StatusCode == http.StatusTooManyRequests:
		delay := 5 * time.Second
		if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
			delay = time.Duration(s) * time.Second
		}
		return nil, temporal.NewApplicationErrorWithOptions("Tavily rate limit", "SearchRateLimited",
			temporal.ApplicationErrorOptions{NextRetryDelay: delay})
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("tavily: %s", resp.Status)
	}

	var out struct {
		Results []struct {
			URL     string  `json:"url"`
			Content string  `json:"content"`
			Score   float64 `json:"score"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("tavily: bad response: %w", err)
	}
	ev := make([]Evidence, 0, len(out.Results))
	for _, r := range out.Results {
		if r.Score < t.MinScore {
			continue
		}
		ev = append(ev, Evidence{Source: r.URL, Snippet: r.Content, Score: r.Score})
	}
	t.mu.Lock()
	t.cache[key] = cachedSearch{at: time.Now(), ev: ev}
	t.mu.Unlock()
	return ev, nil
}
