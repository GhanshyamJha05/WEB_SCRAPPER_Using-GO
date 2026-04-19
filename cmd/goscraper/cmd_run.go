package main

import (
	"fmt"
	"time"

	"github.com/GhanshyamJha05/WEB_SCRAPPER_Using-GO/internal/ui"
	"github.com/GhanshyamJha05/WEB_SCRAPPER_Using-GO/pkg/scraper"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "scrape multiple URLs from a file",
	Example: "  goscraper run --file urls.txt --selector \".titleline > a\"" +
		"\n  goscraper run --file urls.txt --selector \"h2 a\" --workers 8 --output results.json",
	RunE: runRun,
}

var (
	runFile     string
	runSelector string
	runWorkers  int
	runOutput   string
)

func init() {
	runCmd.Flags().StringVar(&runFile, "file", "", "file with one URL per line (required)")
	runCmd.Flags().StringVar(&runSelector, "selector", "", "CSS selector to extract (required)")
	runCmd.Flags().IntVar(&runWorkers, "workers", 6, "number of concurrent workers")
	runCmd.Flags().StringVar(&runOutput, "output", "", "write results to this JSON file (default: stdout)")

	_ = runCmd.MarkFlagRequired("file")
	_ = runCmd.MarkFlagRequired("selector")
}

func runRun(cmd *cobra.Command, _ []string) error {
	urls, err := readURLs(runFile)
	if err != nil {
		ui.Fatal("failed to read input file: " + err.Error())
	}
	if len(urls) == 0 {
		ui.Fatal("no URLs found in " + runFile)
	}

	ui.Header("run")
	ui.Config("input", runFile)
	ui.Config("selector", runSelector)
	ui.Config("workers", fmt.Sprintf("%d", runWorkers))
	if runOutput != "" {
		ui.Config("output", runOutput)
	}
	ui.Spacer()
	ui.Section("fetching")

	cfg := scraper.DefaultConfig()
	cfg.WorkerCount = runWorkers
	cli := scraper.NewClient(cfg)

	// Scrape each URL individually so we can show per-URL progress.
	total := len(urls)
	var allResults []scraper.ScrapeResult
	var allErrs []error

	start := time.Now()
	for i, u := range urls {
		t0 := time.Now()
		results, errs := cli.ScrapeWithWorkerPool([]string{u}, runSelector)
		durationMs := time.Since(t0).Milliseconds()

		var urlErr error
		if len(errs) > 0 {
			urlErr = errs[0]
			allErrs = append(allErrs, errs...)
		}
		allResults = append(allResults, results...)
		ui.Progress(i+1, total, u, len(results), durationMs, urlErr)
	}
	elapsed := time.Since(start).Seconds()

	ui.Done(len(allResults), len(allErrs), elapsed)

	out := scraper.NewScrapeOutput(urls, runSelector, runWorkers, allResults, allErrs)
	if err := writeOutput(runOutput, out); err != nil {
		ui.Fatal("failed to write output: " + err.Error())
	}
	if runOutput != "" {
		ui.Saved(runOutput)
	}
	return nil
}
