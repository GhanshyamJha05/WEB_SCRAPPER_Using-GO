package main

import (
	"fmt"
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

func runFetch(cmd *cobra.Command, _ []string) error {
	ui.Header("fetch")
	ui.Config("url", fetchURL)
	ui.Config("selector", fetchSelector)
	ui.Config("workers", fmt.Sprintf("%d", fetchWorkers))
	if fetchOutput != "" {
		ui.Config("output", fetchOutput)
	}
	ui.Spacer()
	ui.Section("fetching")

	cfg := scraper.DefaultConfig()
	cfg.WorkerCount = fetchWorkers
	cli := scraper.NewClient(cfg)

	start := time.Now()
	results, errs := cli.ScrapeWithWorkerPool([]string{fetchURL}, fetchSelector)
	elapsed := time.Since(start).Seconds()

	// Single URL — always [1/1]
	var fetchErr error
	if len(errs) > 0 {
		fetchErr = errs[0]
	}
	durationMs := int64(time.Since(start).Milliseconds())
	ui.Progress(1, 1, fetchURL, len(results), durationMs, fetchErr)
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
