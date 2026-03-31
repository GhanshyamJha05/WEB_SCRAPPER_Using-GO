// Package scraper implements concurrent CSS-selector web scraping used by the HTTP server and CLI.
package scraper

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// ScrapeResult is one matched element (title text + resolved link).
type ScrapeResult struct {
	Title string `json:"title"`
	Link  string `json:"link"`
}

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

// BulkScrapeRequest is the JSON body for bulk API / CLI bulk mode.
type BulkScrapeRequest struct {
	URLs     []string `json:"urls"`
	Selector string   `json:"selector"`
}

// BulkScrapeResult is one row in a bulk response.
type BulkScrapeResult struct {
	URL             string `json:"url"`
	Data            string `json:"data"`
	ExecutionTimeMs int64  `json:"execution_time_ms"`
	Status          string `json:"status"`
}

// BulkScrapeResponse aggregates timed bulk scrape results.
type BulkScrapeResponse struct {
	TotalBatchTimeMs int                `json:"total_batch_time_ms"`
	Results          []BulkScrapeResult `json:"results"`
}

// Config holds tunables for the worker pool and HTTP client.
type Config struct {
	WorkerCount       int
	RequestInterval   time.Duration
	MaxURLsPerRequest int
	HTTPTimeout       time.Duration
}

// DefaultConfig matches the server defaults.
func DefaultConfig() Config {
	return Config{
		WorkerCount:       6,
		RequestInterval:   200 * time.Millisecond,
		MaxURLsPerRequest: 25,
		HTTPTimeout:       12 * time.Second,
	}
}

// Client performs HTTP fetches and CSS selection with a worker pool.
type Client struct {
	httpClient *http.Client
	cfg        Config
}

// NewClient returns a scraper with the given config (use DefaultConfig() for defaults).
func NewClient(cfg Config) *Client {
	if cfg.HTTPTimeout <= 0 {
		cfg.HTTPTimeout = 12 * time.Second
	}
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 6
	}
	if cfg.RequestInterval <= 0 {
		cfg.RequestInterval = 200 * time.Millisecond
	}
	if cfg.MaxURLsPerRequest <= 0 {
		cfg.MaxURLsPerRequest = 25
	}
	return &Client{
		httpClient: &http.Client{Timeout: cfg.HTTPTimeout},
		cfg:        cfg,
	}
}

func (c *Client) scrapeWebsite(ctx context.Context, pageURL, selector string) ([]ScrapeResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code error: %d %s", res.StatusCode, res.Status)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, err
	}

	var results []ScrapeResult
	doc.Find(selector).Each(func(i int, s *goquery.Selection) {
		title := strings.TrimSpace(s.Text())
		if title == "" {
			return
		}

		link, _ := s.Attr("href")
		if link != "" && !strings.HasPrefix(link, "mailto:") {
			if baseURL, parseErr := url.Parse(pageURL); parseErr == nil {
				if href, hrefErr := url.Parse(link); hrefErr == nil {
					link = baseURL.ResolveReference(href).String()
				}
			}
		}

		results = append(results, ScrapeResult{Title: title, Link: link})
	})

	return results, nil
}

func (c *Client) worker(jobs <-chan scrapeJob, results chan<- jobResult, limiter <-chan time.Time) {
	for job := range jobs {
		<-limiter
		start := time.Now()
		items, err := c.scrapeWebsite(context.Background(), job.url, job.selector)
		results <- jobResult{
			index:      job.index,
			url:        job.url,
			items:      items,
			durationMs: time.Since(start).Milliseconds(),
			err:        err,
		}
	}
}

// ParseURLs splits a raw string into distinct URLs (comma, whitespace, newline).
func ParseURLs(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		u := strings.TrimSpace(p)
		if u == "" {
			continue
		}
		out = append(out, u)
	}
	return out
}

// ScrapeWithWorkerPool runs multiple URLs with the same selector (combined results for UI single view).
func (c *Client) ScrapeWithWorkerPool(urls []string, selector string) ([]ScrapeResult, []error) {
	jobs := make(chan scrapeJob, len(urls))
	results := make(chan jobResult, len(urls))

	limiter := time.NewTicker(c.cfg.RequestInterval)
	defer limiter.Stop()

	activeWorkers := c.cfg.WorkerCount
	if len(urls) < activeWorkers {
		activeWorkers = len(urls)
	}
	if activeWorkers == 0 {
		return nil, nil
	}

	var wg sync.WaitGroup
	for i := 0; i < activeWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.worker(jobs, results, limiter.C)
		}()
	}

	for _, u := range urls {
		jobs <- scrapeJob{url: u, selector: selector}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	combined := make([]ScrapeResult, 0)
	errs := make([]error, 0)
	for result := range results {
		if result.err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", result.url, result.err))
			continue
		}
		combined = append(combined, result.items...)
	}
	return combined, errs
}

// RunBulkScrape executes one timed scrape per URL with stable ordering in Results.
func (c *Client) RunBulkScrape(urls []string, selector string) BulkScrapeResponse {
	start := time.Now()
	response := BulkScrapeResponse{
		Results: make([]BulkScrapeResult, len(urls)),
	}

	jobs := make(chan scrapeJob, len(urls))
	results := make(chan jobResult, len(urls))
	limiter := time.NewTicker(c.cfg.RequestInterval)
	defer limiter.Stop()

	activeWorkers := c.cfg.WorkerCount
	if len(urls) < activeWorkers {
		activeWorkers = len(urls)
	}
	if activeWorkers == 0 {
		return response
	}

	var wg sync.WaitGroup
	for i := 0; i < activeWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.worker(jobs, results, limiter.C)
		}()
	}

	for i, u := range urls {
		jobs <- scrapeJob{index: i, url: u, selector: selector}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		extracted := make([]string, 0, len(result.items))
		for _, item := range result.items {
			if trimmed := strings.TrimSpace(item.Title); trimmed != "" {
				extracted = append(extracted, trimmed)
			}
		}
		row := BulkScrapeResult{
			URL:             result.url,
			Data:            strings.Join(extracted, " | "),
			ExecutionTimeMs: result.durationMs,
			Status:          "success",
		}
		if result.err != nil {
			row.Data = result.err.Error()
			row.Status = "failed"
		}
		response.Results[result.index] = row
	}

	response.TotalBatchTimeMs = int(time.Since(start).Milliseconds())
	return response
}

// MaxURLs returns the configured cap for one request.
func (c *Client) MaxURLs() int { return c.cfg.MaxURLsPerRequest }
