// Package handler is the Vercel serverless entrypoint.
// All handler logic is inlined here because Go's internal package rule
// prevents importing internal/* from outside the module root, which is
// how Vercel's @vercel/go builder compiles this file.
package handler

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/GhanshyamJha05/WEB_SCRAPPER_Using-GO/pkg/scraper"
)

//go:embed templates/index.html
var templateFS embed.FS

// --- types ---

type scrapingSite struct {
	URL      string
	Tag      string
	Selector string
	Example  string
}

type pageData struct {
	URL         string
	Selector    string
	Results     []scraper.ScrapeResult
	Duration    time.Duration
	Error       string
	Recommended []scrapingSite
	Visited     []string
}

// --- state (shared across warm lambda invocations) ---

var (
	tmpl             *template.Template
	cli              *scraper.Client
	mu               sync.Mutex
	visited          []string
	recommendedSites = []scrapingSite{
		{URL: "https://news.ycombinator.com", Tag: "Tech News", Selector: ".titleline > a", Example: "Hacker News headlines"},
		{URL: "https://www.reddit.com/r/golang/", Tag: "Golang", Selector: "h3._eYtD2XCVieq6emjKBH3m", Example: "Reddit post titles"},
		{URL: "https://github.com/trending", Tag: "GitHub", Selector: "h2 a", Example: "Trending repositories"},
	}
)

func init() {
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}
	var err error
	tmpl, err = template.New("index.html").Funcs(funcMap).ParseFS(templateFS, "templates/index.html")
	if err != nil {
		log.Fatalf("failed to parse template: %v", err)
	}
	cli = scraper.NewClient(scraper.DefaultConfig())
}

// --- visited URL helpers ---

func addToVisited(url string) {
	mu.Lock()
	defer mu.Unlock()
	for i, u := range visited {
		if u == url {
			visited = append(visited[:i], visited[i+1:]...)
			visited = append(visited, url)
			return
		}
	}
	visited = append(visited, url)
	if len(visited) > 10 {
		visited = visited[1:]
	}
}

func getVisited() []string {
	mu.Lock()
	defer mu.Unlock()
	copied := make([]string, len(visited))
	copy(copied, visited)
	for i, j := 0, len(copied)-1; i < j; i, j = i+1, j-1 {
		copied[i], copied[j] = copied[j], copied[i]
	}
	return copied
}

// --- Vercel entrypoint ---

// Handler is the exported function Vercel calls for every request.
func Handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/bulk-scrape" || strings.HasSuffix(r.URL.Path, "/bulk-scrape") {
		bulkScrapeHandler(w, r)
		return
	}
	indexHandler(w, r)
}

// --- route handlers ---

func indexHandler(w http.ResponseWriter, r *http.Request) {
	data := pageData{
		Recommended: recommendedSites,
		Visited:     getVisited(),
	}

	rawURL := r.URL.Query().Get("url")
	selector := r.URL.Query().Get("selector")

	if rawURL != "" {
		data.URL = rawURL
		data.Selector = selector
		urls := scraper.ParseURLs(rawURL)

		if len(urls) == 0 {
			data.Error = "Please provide at least one valid URL."
			render(w, data)
			return
		}
		if len(urls) > cli.MaxURLs() {
			data.Error = fmt.Sprintf("Too many URLs. Maximum allowed per request is %d.", cli.MaxURLs())
			render(w, data)
			return
		}
		for _, u := range urls {
			addToVisited(u)
		}

		// Auto-fill selector from recommended sites if not provided.
		if selector == "" {
			for _, site := range recommendedSites {
				if site.URL == urls[0] {
					selector = site.Selector
					data.Selector = selector
					break
				}
			}
		}

		if selector != "" {
			start := time.Now()
			results, errs := cli.ScrapeWithWorkerPool(urls, selector)
			data.Duration = time.Since(start).Round(time.Millisecond)
			if len(errs) > 0 {
				msgs := make([]string, 0, len(errs))
				for _, e := range errs {
					msgs = append(msgs, e.Error())
				}
				data.Error = fmt.Sprintf("Completed with %d error(s): %s", len(errs), strings.Join(msgs, " | "))
			}
			data.Results = results
		} else {
			data.Error = "Please provide a CSS selector."
		}
	}

	render(w, data)
}

func bulkScrapeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload scraper.BulkScrapeRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	urls := make([]string, 0, len(payload.URLs))
	for _, raw := range payload.URLs {
		if u := strings.TrimSpace(raw); u != "" {
			urls = append(urls, u)
		}
	}
	selector := strings.TrimSpace(payload.Selector)

	if len(urls) == 0 {
		http.Error(w, "At least one URL is required", http.StatusBadRequest)
		return
	}
	if selector == "" {
		http.Error(w, "CSS selector is required", http.StatusBadRequest)
		return
	}
	if len(urls) > cli.MaxURLs() {
		http.Error(w, fmt.Sprintf("Maximum %d URLs allowed per request", cli.MaxURLs()), http.StatusBadRequest)
		return
	}

	for _, u := range urls {
		addToVisited(u)
	}

	resp := cli.RunBulkScrape(urls, selector)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func render(w http.ResponseWriter, data pageData) {
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
