package scraper

import (
	"encoding/json"
	"os"
	"time"
)

// ScrapeOutput is the top-level structure written to a JSON file.
// It wraps results with metadata so the file is self-describing.
type ScrapeOutput struct {
	// Meta describes the scrape run that produced this file.
	Meta ScrapeMeta `json:"meta"`

	// Results holds every element matched across all scraped URLs.
	Results []ScrapeResult `json:"results"`

	// Errors lists per-URL errors that occurred during the run (may be empty).
	Errors []string `json:"errors,omitempty"`
}

// ScrapeMeta carries context about the scrape run.
type ScrapeMeta struct {
	ScrapedAt   time.Time `json:"scraped_at"`   // UTC timestamp when the run started
	Selector    string    `json:"selector"`     // CSS selector that was applied
	URLs        []string  `json:"urls"`         // input URLs (in order)
	TotalURLs   int       `json:"total_urls"`   // len(URLs)
	TotalItems  int       `json:"total_items"`  // len(Results)
	TotalErrors int       `json:"total_errors"` // len(Errors)
	Workers     int       `json:"workers"`      // worker goroutines used
}

// NewScrapeOutput builds a ScrapeOutput from the raw scrape results.
func NewScrapeOutput(urls []string, selector string, workers int, results []ScrapeResult, errs []error) ScrapeOutput {
	errStrings := make([]string, 0, len(errs))
	for _, e := range errs {
		errStrings = append(errStrings, e.Error())
	}

	return ScrapeOutput{
		Meta: ScrapeMeta{
			ScrapedAt:   time.Now().UTC(),
			Selector:    selector,
			URLs:        urls,
			TotalURLs:   len(urls),
			TotalItems:  len(results),
			TotalErrors: len(errs),
			Workers:     workers,
		},
		Results: results,
		Errors:  errStrings,
	}
}

// SaveJSON writes out as indented JSON to the given file path.
// The file is created (or truncated) with mode 0644.
func (o ScrapeOutput) SaveJSON(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(o)
}
