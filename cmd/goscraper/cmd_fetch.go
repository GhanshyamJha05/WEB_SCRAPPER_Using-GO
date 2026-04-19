package main

import (
	"time"

	"github.com/GhanshyamJha05/WEB_SCRAPPER_Using-GO/internal/ui"
	"github.com/GhanshyamJha05/WEB_SCRAPPER_Using-GO/pkg/scraper"
	"github.com/spf13/cobra"
)

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "scrape a single URL",
	Example: "  goscraper fetch --url https://news.ycombinator.com --selector \".titleline > a\"" +
		"\n  goscraper fetch --url https://example.com --selector \"h2 a\" --output results.json",
	RunE: runFetch,
}

var (
	fetchURL      string
	fetchSelector string
	fetchWorkers  int
	fetchOutput   string
)

func init() {
	fetchCmd.Flags().StringVar(&fetchURL, "url", "", "URL to scrape (required)")
	fetchCmd.Flags().StringVar(&fetchSelector, "selector", "", "CSS selector to extract (required)")
	fetchCmd.Flags().IntVar(&fetchWorkers, "workers", 6, "number of concurrent workers")
	fetchCmd.Flags().StringVar(&fetchOutput, "output", "", "write results to this JSON file (default: stdout)")

	_ = fetchCmd.MarkFlagRequired("url")
	_ = fetchCmd.MarkFlagRequired("selector")
}

func runFetch(_ *cobra.Command, _ []string) error {
	ui.Header("fetch")
	ui.Config("url", fetchURL)
	ui.Config("selector", fetchSelector)
	if fetchOutput != "" {
		ui.Config("output", fetchOutput)
	}
	ui.Spacer()
	ui.Section("fetching")

	cfg := scraper.DefaultConfig()
	cfg.WorkerCount = fetchWorkers
	cli := scraper.NewClient(cfg)

	// Single URL goes through the same ScrapeStreamed pipeline as run —
	// no special-casing, no duplicated logic.
	start := time.Now()
	var results []scraper.ScrapeResult
	var errs []error

	for r := range cli.ScrapeStreamed([]string{fetchURL}, fetchSelector) {
		if r.Err != nil {
			errs = append(errs, r.Err)
		} else {
			results = append(results, r.Items...)
		}
		ui.Progress(1, 1, r.URL, len(r.Items), r.DurationMs, r.Err)
	}
	elapsed := time.Since(start).Seconds()

	ui.Done(len(results), len(errs), elapsed)

	for _, e := range errs {
		ui.Warn(e.Error())
	}

	out := scraper.NewScrapeOutput([]string{fetchURL}, fetchSelector, fetchWorkers, results, errs)
	if err := writeOutput(fetchOutput, out); err != nil {
		ui.Fatal("failed to write output: " + err.Error())
	}
	if fetchOutput != "" {
		ui.Saved(fetchOutput)
	}
	return nil
}
