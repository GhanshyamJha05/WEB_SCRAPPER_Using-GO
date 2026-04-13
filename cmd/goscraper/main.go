// Command goscraper is a CLI for scraping URLs with a CSS selector.
//
// Usage:
//
//	goscraper --input urls.txt --selector ".title a" [--workers 8] [--output results.json]
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/GhanshyamJha05/WEB_SCRAPPER_Using-GO/pkg/scraper"
)

func main() {
	input := flag.String("input", "", "Path to file with one URL per line (required)")
	selector := flag.String("selector", "", "CSS selector to extract (required)")
	workers := flag.Int("workers", 6, "Number of concurrent worker goroutines")
	output := flag.String("output", "", "Write JSON results to this file (default: stdout)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "goscraper — concurrent web scraper\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  goscraper --input <file> --selector <css> [--workers N] [--output <file>]\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  goscraper --input urls.txt --selector \".titleline > a\" --workers 8 --output out.json\n")
	}

	flag.Parse()

	if *input == "" || *selector == "" {
		fmt.Fprintln(os.Stderr, "error: --input and --selector are required")
		flag.Usage()
		os.Exit(1)
	}
	if *workers < 1 {
		fmt.Fprintln(os.Stderr, "error: --workers must be >= 1")
		os.Exit(1)
	}

	urls, err := readURLs(*input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading input: %v\n", err)
		os.Exit(1)
	}
	if len(urls) == 0 {
		fmt.Fprintln(os.Stderr, "error: no URLs found in input file")
		os.Exit(1)
	}

	cfg := scraper.DefaultConfig()
	cfg.WorkerCount = *workers

	cli := scraper.NewClient(cfg)
	results, errs := cli.ScrapeWithWorkerPool(urls, *selector)

	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "warn: %v\n", e)
	}

	if err := writeOutput(*output, results); err != nil {
		fmt.Fprintf(os.Stderr, "error writing output: %v\n", err)
		os.Exit(1)
	}
}

// readURLs reads a file and returns non-empty, non-comment lines as URLs.
// It strips a leading UTF-8 BOM if present.
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
			// strip UTF-8 BOM (EF BB BF) written by some editors
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

// writeOutput encodes results as indented JSON to a file or stdout.
func writeOutput(path string, results []scraper.ScrapeResult) error {
	w := os.Stdout
	if path != "" {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}
