package httperr

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"go.temporal.io/sdk/temporal"
)

// StatusError turns a non-200 provider response into a Temporal error, or nil
// for 200. A rejected key or request can never succeed on retry, so it is
// non-retryable; a 429 retries after the provider's Retry-After.
func StatusError(service string, resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return temporal.NewNonRetryableApplicationError(service+" rejected the API key", "ProviderAuth", nil)
	case http.StatusTooManyRequests:
		delay := 5 * time.Second
		if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
			delay = time.Duration(s) * time.Second
		}
		return temporal.NewApplicationErrorWithOptions(service+" rate limit", "ProviderRateLimited",
			temporal.ApplicationErrorOptions{NextRetryDelay: delay})
	case http.StatusBadRequest, http.StatusNotFound, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("%s rejected the request: %s: %s", service, resp.Status, body), "ProviderBadRequest", nil)
	default:
		return fmt.Errorf("%s: %s", service, resp.Status)
	}
}
