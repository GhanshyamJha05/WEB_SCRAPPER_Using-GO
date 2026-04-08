package scraper

import (
	"errors"
	"net/http"
	"time"
)

// retryableError wraps the last error after all attempts are exhausted.
type retryableError struct {
	attempts int
	err      error
}

func (e *retryableError) Error() string { return e.err.Error() }
func (e *retryableError) Unwrap() error { return e.err }

// isRetryable returns true for errors worth retrying:
//   - any network/timeout error from http.Client.Do
//   - HTTP 429 Too Many Requests
//   - HTTP 5xx server errors
func isRetryable(err error, statusCode int) bool {
	if err != nil {
		return true // covers timeouts, connection resets, DNS failures
	}
	return statusCode == http.StatusTooManyRequests || statusCode >= 500
}

// withRetry calls do up to maxRetries times with exponential backoff between attempts.
//
// Backoff schedule (baseDelay = 300ms):
//
//	attempt 0 fails → wait 300ms  → attempt 1
//	attempt 1 fails → wait 600ms  → attempt 2
//	attempt 2 fails → wait 1200ms → attempt 3  (last)
//
// Returns immediately on the first success or non-retryable error.
// Respects Retry-After header on 429 responses.
func withRetry(maxRetries int, baseDelay time.Duration, do func() (*http.Response, error)) (*http.Response, error) {
	var (
		resp *http.Response
		err  error
	)

	for attempt := range maxRetries {
		resp, err = do()

		// Success — no error and status is not retryable.
		if err == nil && !isRetryable(nil, resp.StatusCode) {
			return resp, nil
		}

		// Close body before retry to avoid leaking connections.
		if resp != nil {
			resp.Body.Close()
		}

		// Last attempt — don't sleep, fall through to return the error.
		if attempt == maxRetries-1 {
			break
		}

		// Exponential backoff: baseDelay * 2^attempt
		sleep := baseDelay * (1 << attempt)

		// Honour Retry-After header if the server sent one (common with 429).
		if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, parseErr := time.ParseDuration(ra + "s"); parseErr == nil {
					sleep = secs
				}
			}
		}

		time.Sleep(sleep)
	}

	// Wrap the last error with attempt count for observability.
	lastErr := err
	if lastErr == nil {
		lastErr = errors.New(resp.Status)
	}
	return nil, &retryableError{attempts: maxRetries, err: lastErr}
}
