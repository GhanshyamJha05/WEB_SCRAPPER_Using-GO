// Package server contains the HTTP handler logic for the web scraper UI.
// It is internal to this module and not intended for external import.
package server

import (
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

// ScrapingSite is a pre-configured site shown as a recommendation in the UI.
type ScrapingSite struct {
	URL      string
	Tag      string
	Selector string
	Example  string
}

// PageData is the template context for the index page.
type PageData struct {
	URL         string
	Selector    string
	Results     []scraper.ScrapeResult
	Duration    time.Duration
	Error       string
	Recommended []ScrapingSite
	Visited     []string
}

// RecommendedSites are the default suggestions shown in the UI.
var RecommendedSites = []ScrapingSite{
	{URL: "https://news.ycombinator.com", Tag: "Tech News", Selector: ".titleline > a", Example: "Hacker News headlines"},
	{URL: "https://www.reddit.com/r/golang/", Tag: "Golang", Selector: "h3._eYtD2XCVieq6emjKBH3m", Example: "Reddit post titles"},
	{URL: "https://github.com/trending", Tag: "GitHub", Selector: "h2 a", Example: "Trending repositories"},
}

// Handler holds shared state and handles HTTP requests.
type Handler struct {
	tmpl    *template.Template
	cli     *scraper.Client
	mu      sync.Mutex
	visited []string
}

// New creates a Handler with the given template and scraper client.
func New(tmpl *template.Template, cli *scraper.Client) *Handler {
	return &Handler{tmpl: tmpl, cli: cli}
}

func (h *Handler) addToVisited(url string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, u := range h.visited {
		if u == url {
			h.visited = append(h.visited[:i], h.visited[i+1:]...)
			h.visited = append(h.visited, url)
			return
		}
	}
	h.visited = append(h.visited, url)
	if len(h.visited) > 10 {
		h.visited = h.visited[1:]
	}
}

func (h *Handler) getVisited() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	copied := make([]string, len(h.visited))
	copy(copied, h.visited)
	for i, j := 0, len(copied)-1; i < j; i, j = i+1, j-1 {
		copied[i], copied[j] = copied[j], copied[i]
	}
	return copied
}

// ServeHTTP routes requests to the appropriate handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/bulk-scrape" || strings.HasSuffix(r.URL.Path, "/bulk-scrape") {
		h.BulkScrape(w, r)
		return
	}
	h.Index(w, r)
}

// Index handles the main scraper UI page (GET /).
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Recommended: RecommendedSites,
		Visited:     h.getVisited(),
	}

	rawURL := r.URL.Query().Get("url")
	selector := r.URL.Query().Get("selector")

	if rawURL != "" {
		data.URL = rawURL
		data.Selector = selector
		urls := scraper.ParseURLs(rawURL)

		if len(urls) == 0 {
			data.Error = "Please provide at least one valid URL."
			h.render(w, data)
			return
		}
		if len(urls) > h.cli.MaxURLs() {
			data.Error = fmt.Sprintf("Too many URLs. Maximum allowed per request is %d.", h.cli.MaxURLs())
			h.render(w, data)
			return
		}
		for _, u := range urls {
			h.addToVisited(u)
		}

		// Auto-fill selector from recommended sites if not provided.
		if selector == "" {
			for _, site := range RecommendedSites {
				if site.URL == urls[0] {
					selector = site.Selector
					data.Selector = selector
					break
				}
			}
		}

		if selector != "" {
			start := time.Now()
			results, errs := h.cli.ScrapeWithWorkerPool(urls, selector)
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

	h.render(w, data)
}

// BulkScrape handles POST /api/bulk-scrape.
func (h *Handler) BulkScrape(w http.ResponseWriter, r *http.Request) {
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
	if len(urls) > h.cli.MaxURLs() {
		http.Error(w, fmt.Sprintf("Maximum %d URLs allowed per request", h.cli.MaxURLs()), http.StatusBadRequest)
		return
	}

	for _, u := range urls {
		h.addToVisited(u)
	}

	resp := h.cli.RunBulkScrape(urls, selector)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) render(w http.ResponseWriter, data PageData) {
	if err := h.tmpl.Execute(w, data); err != nil {
		log.Printf("template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
