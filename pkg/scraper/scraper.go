// Package scraper implements concurrent CSS-selector web scraping.
package scraper

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// ScrapeResult is one matched element: its text and resolved href.
type ScrapeResult struct {
	Title string `json:"title"`
	Link  string `json:"link"`
}

// internal job/result types passed through the worker pool channels.
type scrapeJob struct {
	index    int
	url      string
	selector string
}

type jobResult struct {
	index      int
	url        string
	items      []ScrapeResult
	durationMs int64
	err        error
}

// --- Public request/response types used by the HTTP API and CLI ---

// BulkScrapeRequest is the JSON body for POST /api/bulk-scrape.
type BulkScrapeRequest struct {
	URLs     []string `json:"urls"`
	Selector string   `json:"selector"`
}

// BulkScrapeResult is one row in a BulkScrapeResponse.
type BulkScrapeResult struct {
	URL             string `json:"url"`
	Data            string `json:"data"`
	ExecutionTimeMs int64  `json:"execution_time_ms"`
	Status          string `json:"status"`
}

// BulkScrapeResponse is the full response for a bulk scrape run.
type BulkScrapeResponse struct {
	TotalBatchTimeMs int                `json:"total_batch_time_ms"`
	Results          []BulkScrapeResult `json:"results"`
}

// --- Config & Client ---

// Config holds tunables for the worker pool and HTTP client.
type Config struct {
	WorkerCount       int           // number of concurrent worker goroutines
	RateLimit         float64       // maximum requests per second across all workers
	MaxURLsPerRequest int           // hard cap on URLs per call
	HTTPTimeout       time.Duration // per-request HTTP timeout
	MaxRetries        int           // max retry attempts on failure (0 = no retries)
	BaseRetryDelay    time.Duration // initial backoff delay; doubles each attempt
}

// DefaultConfig returns sensible production defaults.
func DefaultConfig() Config {
	return Config{
		WorkerCount:       6,
		RateLimit:         5,
		MaxURLsPerRequest: 25,
		HTTPTimeout:       12 * time.Second,
		MaxRetries:        3,
		BaseRetryDelay:    300 * time.Millisecond,
	}
}

// Client performs HTTP fetches and CSS selection using a worker pool.
type Client struct {
	httpClient *http.Client
	cfg        Config
}

// NewClient returns a Client with validated config values.
func NewClient(cfg Config) *Client {
	if cfg.HTTPTimeout <= 0 {
		cfg.HTTPTimeout = 12 * time.Second
	}
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 6
	}
	if cfg.RateLimit <= 0 {
		cfg.RateLimit = 5
	}
	if cfg.MaxURLsPerRequest <= 0 {
		cfg.MaxURLsPerRequest = 25
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.BaseRetryDelay <= 0 {
		cfg.BaseRetryDelay = 300 * time.Millisecond
	}
	return &Client{
		httpClient: &http.Client{Timeout: cfg.HTTPTimeout},
		cfg:        cfg,
	}
}

// MaxURLs returns the configured cap for one request.
func (c *Client) MaxURLs() int { return c.cfg.MaxURLsPerRequest }

// --- Core fetch logic ---

// fetch performs an HTTP GET with automatic retry + exponential backoff.
// It retries on network errors, timeouts, 429, and 5xx responses (up to maxRetries).
// This is the fetchFn passed to the worker pool.
func (c *Client) fetch(ctx context.Context, pageURL, selector string) ([]ScrapeResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}

	res, err := withRetry(c.cfg.MaxRetries, c.cfg.BaseRetryDelay, func() (*http.Response, error) {
		return c.httpClient.Do(req)
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", pageURL, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d %s", res.StatusCode, res.Status)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, err
	}

	var results []ScrapeResult
	doc.Find(selector).Each(func(_ int, s *goquery.Selection) {
		title := strings.TrimSpace(s.Text())
		if title == "" {
			return
		}
		link, _ := s.Attr("href")
		if link != "" && !strings.HasPrefix(link, "mailto:") {
			if base, err := url.Parse(pageURL); err == nil {
				if href, err := url.Parse(link); err == nil {
					link = base.ResolveReference(href).String()
				}
			}
		}
		results = append(results, ScrapeResult{Title: title, Link: link})
	})
	return results, nil
}

// --- Public scraping methods ---

// JobResult is one completed URL delivered by ScrapeStreamed.
// It carries everything the CLI needs to call ui.Progress() immediately.
type JobResult struct {
	URL        string
	Items      []ScrapeResult
	DurationMs int64
	Err        error
}

// ScrapeStreamed submits all URLs to the worker pool at once and returns a
// read-only channel that emits one JobResult per URL as each finishes.
// The channel is closed automatically when all workers are done.
//
// Usage:
//
//	for r := range cli.ScrapeStreamed(urls, selector) {
//	    ui.Progress(n, total, r.URL, len(r.Items), r.DurationMs, r.Err)
//	}
func (c *Client) ScrapeStreamed(urls []string, selector string) <-chan JobResult {
	out := make(chan JobResult, len(urls))

	if len(urls) == 0 {
		close(out)
		return out
	}

	workers := min(c.cfg.WorkerCount, len(urls))
	p := newPool(workers, c.fetch, newRateLimiter(c.cfg.RateLimit))

	// Submit all jobs before starting the drain goroutine so the pool is
	// fully loaded — workers start immediately as jobs arrive.
	go func() {
		for _, u := range urls {
			p.submit(scrapeJob{url: u, selector: selector})
		}
		p.done() // signal no more jobs; workers drain then close p.results
	}()

	// Translate internal jobResults into public JobResults and forward them.
	go func() {
		for r := range p.results {
			out <- JobResult{
				URL:        r.url,
				Items:      r.items,
				DurationMs: r.durationMs,
				Err:        r.err,
			}
		}
		close(out)
	}()

	return out
}

// ScrapeWithWorkerPool scrapes all URLs concurrently and merges results into one slice.
// Errors are collected separately so partial results are still returned.
// For streaming per-URL progress use ScrapeStreamed instead.
func (c *Client) ScrapeWithWorkerPool(urls []string, selector string) ([]ScrapeResult, []error) {
	var combined []ScrapeResult
	var errs []error
	for r := range c.ScrapeStreamed(urls, selector) {
		if r.Err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.URL, r.Err))
			continue
		}
		combined = append(combined, r.Items...)
	}
	return combined, errs
}

// RunBulkScrape scrapes each URL independently and returns per-URL timing and status.
// Results are returned in the same order as the input URLs.
func (c *Client) RunBulkScrape(urls []string, selector string) BulkScrapeResponse {
	start := time.Now()
	resp := BulkScrapeResponse{
		Results: make([]BulkScrapeResult, len(urls)),
	}
	if len(urls) == 0 {
		return resp
	}

	// Build an index so we can place results back in input order.
	index := make(map[string]int, len(urls))
	for i, u := range urls {
		index[u] = i
	}

	for r := range c.ScrapeStreamed(urls, selector) {
		row := BulkScrapeResult{
			URL:             r.URL,
			ExecutionTimeMs: r.DurationMs,
			Status:          "success",
		}
		if r.Err != nil {
			row.Data = r.Err.Error()
			row.Status = "failed"
		} else {
			titles := make([]string, 0, len(r.Items))
			for _, item := range r.Items {
				if t := strings.TrimSpace(item.Title); t != "" {
					titles = append(titles, t)
				}
			}
			row.Data = strings.Join(titles, " | ")
		}
		resp.Results[index[r.URL]] = row // stable ordering via original index
	}

	resp.TotalBatchTimeMs = int(time.Since(start).Milliseconds())
	return resp
}

// ParseURLs splits a raw string on commas, spaces, and newlines into distinct URLs.
func ParseURLs(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if u := strings.TrimSpace(p); u != "" {
			out = append(out, u)
		}
	}
	return out
}
