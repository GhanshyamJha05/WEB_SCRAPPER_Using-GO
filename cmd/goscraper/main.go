package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/GhanshyamJha05/WEB_SCRAPPER_Using-GO/internal/ui"
	"github.com/GhanshyamJha05/WEB_SCRAPPER_Using-GO/pkg/scraper"
	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:     "goscraper",
	Version: version,
	Short:   "a minimal web scraping CLI",
	Long: `A minimal web scraping CLI.

  Available commands:
    fetch      scrape a single URL
    run        scrape multiple URLs from a file
    discover   list links from a page

Use "goscraper [command] --help" for more information.`,
	// Don't show usage on every error — only on flag parse failures.
	SilenceUsage: true,
	// Let RunE return errors without cobra printing them (we handle it).
	SilenceErrors: true,
}

var (
	flagVerbose bool
	flagQuiet   bool
)

func init() {
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "show detailed output including per-URL timing")
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "suppress all output except errors and saved path")

	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		switch {
		case flagVerbose:
			ui.Active = ui.ModeVerbose
		case flagQuiet:
			ui.Active = ui.ModeQuiet
		default:
			ui.Active = ui.ModeNormal
		}
	}

	rootCmd.AddCommand(fetchCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(discoverCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		ui.Fatal(err.Error())
	}
}

// --- shared helpers used by multiple commands ---

// readURLs reads a file and returns non-empty, non-comment lines as URLs.
// Strips a leading UTF-8 BOM written by some Windows editors.
func readURLs(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var urls []string
	sc := bufio.NewScanner(f)
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first {
			line = strings.TrimPrefix(line, "\xef\xbb\xbf")
			first = false
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		urls = append(urls, line)
	}
	return urls, sc.Err()
}

// writeOutput writes a ScrapeOutput as indented JSON to a file or stdout.
func writeOutput(path string, out scraper.ScrapeOutput) error {
	if path != "" {
		return out.SaveJSON(path)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// printVersion is used by --version (cobra handles this automatically).
var _ = fmt.Sprintf // keep fmt imported
