package main

import (
	"fmt"
	"strconv"

	"github.com/GhanshyamJha05/WEB_SCRAPPER_Using-GO/internal/ui"
	"github.com/GhanshyamJha05/WEB_SCRAPPER_Using-GO/pkg/scraper"
	"github.com/spf13/cobra"
)

var discoverCmd = &cobra.Command{
	Use:     "discover",
	Short:   "list links from a page",
	Example: "  goscraper discover --url https://example.com\n  goscraper discover --url https://example.com --limit 10",
	RunE:    runDiscover,
}

var (
	discoverURL   string
	discoverLimit int
)

func init() {
	discoverCmd.Flags().StringVar(&discoverURL, "url", "", "URL to discover links from (required)")
	discoverCmd.Flags().IntVar(&discoverLimit, "limit", 15, "max number of links to show")

	_ = discoverCmd.MarkFlagRequired("url")
}

func runDiscover(_ *cobra.Command, _ []string) error {
	ui.Header("discover")
	ui.Config("url", discoverURL)
	ui.Config("limit", fmt.Sprintf("%d", discoverLimit))
	ui.Spacer()
	ui.Section("fetching")

	cli := scraper.NewClient(scraper.DefaultConfig())
	results, errs := cli.ScrapeWithWorkerPool([]string{discoverURL}, "a[href]")

	if len(errs) > 0 {
		ui.Fatal("failed to fetch page: " + shortMessage(errs[0]))
	}

	// Deduplicate and collect hrefs.
	seen := make(map[string]bool)
	var links []string
	for _, r := range results {
		if r.Link == "" || seen[r.Link] {
			continue
		}
		seen[r.Link] = true
		links = append(links, r.Link)
		if len(links) >= discoverLimit {
			break
		}
	}

	if len(links) == 0 {
		ui.Error("no links found on page")
		return nil
	}

	ui.Links(links)

	// Interactive selection.
	raw := ui.Prompt("select a link (number) or press Enter to skip")
	if raw == "" {
		return nil
	}

	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > len(links) {
		ui.Error(fmt.Sprintf("invalid selection: %q — enter a number between 1 and %d", raw, len(links)))
		return nil
	}

	chosen := links[n-1]
	fmt.Println(chosen) // only stdout output — pipeable
	return nil
}

// shortMessage strips the URL prefix from a scraper error and returns the cause.
func shortMessage(err error) string {
	msg := err.Error()
	for i := 0; i < len(msg)-2; i++ {
		if msg[i] == ':' && msg[i+1] == ' ' {
			return msg[i+2:]
		}
	}
	return msg
}
