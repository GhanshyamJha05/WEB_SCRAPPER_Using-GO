package main

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

	"github.com/GhanshyamJha05/WEB_SCRAPPER_Using-GO/internal/scraper"
)

// version is set at link time (e.g. Docker build -ldflags).
var version = "dev"

//go:embed api/templates/index.html
var templateFS embed.FS

type ScrapingSite struct {
	URL      string
	Tag      string
	Selector string
	Example  string
}

type ScrapeResult = scraper.ScrapeResult

type PageData struct {
	URL         string
	Selector    string
	Results     []ScrapeResult
	Duration    time.Duration
	Error       string
	Recommended []ScrapingSite
	Visited     []string
}

var (
	visitedURLs []string
	mu          sync.Mutex
	tmpl        *template.Template
	scrapeCli   = scraper.NewClient(scraper.DefaultConfig())
)

var recommendedSites = []ScrapingSite{
	{
		URL:      "https://news.ycombinator.com",
		Tag:      "Tech News",
		Selector: ".titleline > a",
		Example:  "Hacker News headlines",
	},
	{
		URL:      "https://www.reddit.com/r/golang/",
		Tag:      "Golang",
		Selector: "h3._eYtD2XCVieq6emjKBH3m",
		Example:  "Reddit post titles",
	},
	{
		URL:      "https://github.com/trending",
		Tag:      "GitHub",
		Selector: "h2 a",
		Example:  "Trending repositories",
	},
}

func init() {
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}
	var err error
	tmpl, err = template.New("index.html").Funcs(funcMap).ParseFS(templateFS, "api/templates/index.html")
	if err != nil {
		log.Fatalf("Error parsing template: %v", err)
	}
}

func addToVisited(url string) {
	mu.Lock()
	defer mu.Unlock()
	for i, u := range visitedURLs {
		if u == url {
			visitedURLs = append(visitedURLs[:i], visitedURLs[i+1:]...)
			visitedURLs = append(visitedURLs, url)
			return
		}
	}
	visitedURLs = append(visitedURLs, url)
	if len(visitedURLs) > 10 {
		visitedURLs = visitedURLs[1:]
	}
}

func getVisited() []string {
	mu.Lock()
	defer mu.Unlock()
	copied := make([]string, len(visitedURLs))
	copy(copied, visitedURLs)
	for i, j := 0, len(copied)-1; i < j; i, j = i+1, j-1 {
		copied[i], copied[j] = copied[j], copied[i]
	}
	return copied
}

func bulkScrapeAPIHandler(w http.ResponseWriter, r *http.Request) {
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
		u := strings.TrimSpace(raw)
		if u == "" {
			continue
		}
		urls = append(urls, u)
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
	if len(urls) > scrapeCli.MaxURLs() {
		http.Error(w, fmt.Sprintf("Maximum %d URLs allowed per request", scrapeCli.MaxURLs()), http.StatusBadRequest)
		return
	}

	for _, u := range urls {
		addToVisited(u)
	}

	response := scrapeCli.RunBulkScrape(urls, selector)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func main() {
	http.HandleFunc("/api/bulk-scrape", bulkScrapeAPIHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data := PageData{
			Recommended: recommendedSites,
			Visited:     getVisited(),
		}

		url := r.URL.Query().Get("url")
		selector := r.URL.Query().Get("selector")

		if url != "" {
			data.URL = url
			data.Selector = selector
			urls := scraper.ParseURLs(url)
			if len(urls) == 0 {
				data.Error = "Please provide at least one valid URL."
				if err := tmpl.Execute(w, data); err != nil {
					log.Printf("Template execution error: %v", err)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
				return
			}
			if len(urls) > scrapeCli.MaxURLs() {
				data.Error = fmt.Sprintf("Too many URLs. Maximum allowed per request is %d.", scrapeCli.MaxURLs())
				if err := tmpl.Execute(w, data); err != nil {
					log.Printf("Template execution error: %v", err)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
				return
			}
			for _, u := range urls {
				addToVisited(u)
			}

			if selector == "" {
				for _, site := range recommendedSites {
					if site.URL == urls[0] {
						data.Selector = site.Selector
						selector = site.Selector
						break
					}
				}
			}

			if selector != "" {
				start := time.Now()
				results, errs := scrapeCli.ScrapeWithWorkerPool(urls, selector)
				data.Duration = time.Since(start).Round(time.Millisecond)

				if len(errs) > 0 {
					errStrings := make([]string, 0, len(errs))
					for _, err := range errs {
						errStrings = append(errStrings, err.Error())
					}
					data.Error = fmt.Sprintf("Completed with %d error(s): %s", len(errs), strings.Join(errStrings, " | "))
				}

				data.Results = results
			} else {
				data.Error = "Please provide a CSS selector."
			}
		}

		if err := tmpl.Execute(w, data); err != nil {
			log.Printf("Template execution error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	})

	fmt.Printf("Web Scraper %s — http://localhost:8080\n", version)
	fmt.Println("Press Ctrl+C to stop")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
