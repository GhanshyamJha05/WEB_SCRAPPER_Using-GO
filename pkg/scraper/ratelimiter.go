package scraper

import "time"

// rateLimiter gates concurrent workers to a maximum global request rate.
// All workers share one limiter; each must call wait() before firing a request.
// This ensures the combined throughput never exceeds RateLimit req/s regardless
// of how many workers are running.
//
// Example: RateLimit=5 → one tick every 200ms → at most 5 requests/sec total.
type rateLimiter struct {
	ticker *time.Ticker
}

// newRateLimiter creates a limiter for the given requests-per-second rate.
// rps must be > 0; values <= 0 default to 1 req/s.
func newRateLimiter(rps float64) *rateLimiter {
	if rps <= 0 {
		rps = 1
	}
	interval := time.Duration(float64(time.Second) / rps)
	return &rateLimiter{ticker: time.NewTicker(interval)}
}

// wait blocks until the limiter allows the next request.
func (r *rateLimiter) wait() {
	<-r.ticker.C
}

// stop releases the underlying ticker resources.
func (r *rateLimiter) stop() {
	r.ticker.Stop()
}
