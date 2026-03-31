// Command goscraper is a production-style CLI for the scraper engine with optional Prometheus metrics.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/GhanshyamJha05/WEB_SCRAPPER_Using-GO/pkg/scraper"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const appVersion = "1.0.0"

var (
	scrapesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "goscraper",
			Name:      "scrapes_total",
			Help:      "Total CLI scrape operations by mode and status.",
		},
		[]string{"mode", "status"},
	)
	scrapeDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "goscraper",
			Name:      "scrape_duration_seconds",
			Help:      "Wall time for single scrape or full bulk batch.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"mode"},
	)
	urlScrapeSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "goscraper",
			Name:      "url_scrape_duration_seconds",
			Help:      "Per-URL duration observed in bulk runs.",
			Buckets:   []float64{.05, .1, .25, .5, 1, 2, 5, 10, 30},
		},
		[]string{"status"},
	)
)

func main() {
	args := os.Args[1:]
	metricsListen := envOr("GOSCRAPER_METRICS_LISTEN", "")
	var rest []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--metrics-listen" && i+1 < len(args) {
			metricsListen = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(args[i], "--metrics-listen=") {
			metricsListen = strings.TrimPrefix(args[i], "--metrics-listen=")
			continue
		}
		rest = append(rest, args[i])
	}

	if len(rest) < 1 {
		usage()
		os.Exit(1)
	}

	if metricsListen != "" {
		startMetricsServer(metricsListen)
	}

	switch rest[0] {
	case "scrape":
		fs := flag.NewFlagSet("scrape", flag.ExitOnError)
		url := fs.String("url", "", "Page URL")
		sel := fs.String("selector", "", "CSS selector")
		asJSON := fs.Bool("json", false, "Output JSON")
		if err := fs.Parse(rest[1:]); err != nil {
			os.Exit(2)
		}
		if *url == "" || *sel == "" {
			fmt.Fprintln(os.Stderr, "scrape: -url and -selector are required")
			os.Exit(2)
		}
		runScrape(*url, *sel, *asJSON)

	case "bulk":
		fs := flag.NewFlagSet("bulk", flag.ExitOnError)
		sel := fs.String("selector", "", "CSS selector")
		urlsFile := fs.String("f", "", "File with one URL per line (# comments)")
		asJSON := fs.Bool("json", true, "Output JSON (default true)")
		if err := fs.Parse(rest[1:]); err != nil {
			os.Exit(2)
		}
		if *sel == "" {
			fmt.Fprintln(os.Stderr, "bulk: -selector is required")
			os.Exit(2)
		}
		var urls []string
		if *urlsFile != "" {
			f, err := os.Open(*urlsFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "bulk: %v\n", err)
				os.Exit(2)
			}
			defer f.Close()
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				line := strings.TrimSpace(sc.Text())
				if line != "" && !strings.HasPrefix(line, "#") {
					urls = append(urls, line)
				}
			}
			if err := sc.Err(); err != nil {
				fmt.Fprintf(os.Stderr, "bulk: %v\n", err)
				os.Exit(2)
			}
		}
		urls = append(urls, fs.Args()...)
		if len(urls) == 0 {
			fmt.Fprintln(os.Stderr, "bulk: pass URLs or use -f file")
			os.Exit(2)
		}
		runBulk(urls, *sel, *asJSON)

	case "version":
		fmt.Printf("goscraper %s\n", appVersion)
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `goscraper — CLI for WEB_SCRAPPER_Using-GO

Usage:
  goscraper [--metrics-listen :9091] scrape   -url <u> -selector <css> [-json]
  goscraper [--metrics-listen :9091] bulk     -selector <css> [-f urls.txt] [url ...]
  goscraper version

Metrics:
  --metrics-listen addr   Serve Prometheus at http://<addr>/metrics (also GOSCRAPER_METRICS_LISTEN)
  GET /healthz            Liveness probe when metrics server is enabled

`)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func registerMetrics(reg prometheus.Registerer) {
	reg.MustRegister(scrapesTotal, scrapeDuration, urlScrapeSeconds)
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
}

func startMetricsServer(addr string) {
	reg := prometheus.NewRegistry()
	registerMetrics(reg)
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	go func() {
		fmt.Fprintf(os.Stderr, "goscraper: metrics on http://%s/metrics\n", addr)
		if err := http.ListenAndServe(addr, mux); err != nil {
			fmt.Fprintf(os.Stderr, "goscraper: metrics server: %v\n", err)
		}
	}()
}

func runScrape(url, selector string, asJSON bool) {
	start := time.Now()
	cli := scraper.NewClient(scraper.DefaultConfig())
	results, errs := cli.ScrapeWithWorkerPool([]string{url}, selector)
	scrapeDuration.WithLabelValues("single").Observe(time.Since(start).Seconds())
	if len(errs) > 0 {
		scrapesTotal.WithLabelValues("single", "error").Inc()
		fmt.Fprintln(os.Stderr, errs[0].Error())
		os.Exit(1)
	}
	scrapesTotal.WithLabelValues("single", "ok").Inc()
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(results)
		return
	}
	for i, r := range results {
		fmt.Printf("%02d  %s\n    %s\n", i+1, r.Title, r.Link)
	}
}

func runBulk(urls []string, selector string, asJSON bool) {
	cli := scraper.NewClient(scraper.DefaultConfig())
	if len(urls) > cli.MaxURLs() {
		fmt.Fprintf(os.Stderr, "bulk: at most %d URLs\n", cli.MaxURLs())
		os.Exit(2)
	}
	start := time.Now()
	resp := cli.RunBulkScrape(urls, selector)
	scrapeDuration.WithLabelValues("bulk").Observe(time.Since(start).Seconds())
	scrapesTotal.WithLabelValues("bulk", "ok").Inc()
	for _, row := range resp.Results {
		st := "success"
		if row.Status != "success" {
			st = "failed"
		}
		urlScrapeSeconds.WithLabelValues(st).Observe(float64(row.ExecutionTimeMs) / 1000.0)
	}
	if !asJSON {
		fmt.Printf("total_batch_time_ms: %d\n", resp.TotalBatchTimeMs)
		for _, row := range resp.Results {
			fmt.Printf("- %s [%s] %dms %s\n", row.URL, row.Status, row.ExecutionTimeMs, row.Data)
		}
		return
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp)
}
