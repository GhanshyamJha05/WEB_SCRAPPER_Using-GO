package scraper

import (
	"context"
	"sync"
	"time"
)

// pool manages a fixed set of worker goroutines that drain a jobs channel.
type pool struct {
	jobs    chan scrapeJob
	results chan jobResult
	wg      sync.WaitGroup
}

// newPool starts `workers` goroutines immediately.
// Each worker pulls a job, calls rl.wait() to honour the rate limit, then fetches.
// Call submit() to enqueue work, done() to signal no more jobs, then range results.
func newPool(workers int, fetch fetchFn, rl *rateLimiter) *pool {
	p := &pool{
		// Unbuffered: workers block until a job is available (natural backpressure).
		jobs: make(chan scrapeJob),
		// Buffered: workers never block writing results back.
		results: make(chan jobResult, workers*2),
	}

	for range workers {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for job := range p.jobs {
				rl.wait() // honour global rate limit before each request
				start := time.Now()
				items, err := fetch(context.Background(), job.url, job.selector)
				p.results <- jobResult{
					index:      job.index,
					url:        job.url,
					items:      items,
					durationMs: time.Since(start).Milliseconds(),
					err:        err,
				}
			}
		}()
	}

	// Close results once all workers finish — callers can range over p.results.
	go func() {
		p.wg.Wait()
		rl.stop()
		close(p.results)
	}()

	return p
}

// submit enqueues a job. Must not be called after done().
func (p *pool) submit(job scrapeJob) { p.jobs <- job }

// done signals no more jobs. Workers exit after draining remaining jobs.
func (p *pool) done() { close(p.jobs) }

// fetchFn is the function workers call to fetch and parse a single page.
type fetchFn func(ctx context.Context, pageURL, selector string) ([]ScrapeResult, error)
