package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Data structures
type ScrapingSite struct {
	URL      string
	Tag      string
	Selector string
	Example  string
}

type ScrapeResult struct {
	Title string
	Link  string
}

type PageData struct {
	URL         string
	Selector    string
	Results     []ScrapeResult
	Duration    time.Duration
	Error       string
	Recommended []ScrapingSite
	Visited     []string
}

// Global state (Thread-safe)
var (
	visitedURLs []string
	mu          sync.Mutex
	tmpl        *template.Template
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
	// Register template functions
	funcMap := template.FuncMap{
		"add": func(a, b int) int {
			return a + b
		},
	}

	// Parse templates on startup
	var err error
	tmpl, err = template.New("index.html").Funcs(funcMap).ParseFiles("templates/index.html")
	if err != nil {
		log.Fatalf("Error parsing template: %v", err)
	}
}

func addToVisited(url string) {
	mu.Lock()
	defer mu.Unlock()

	// Check if already exists (move to top?)
	for i, u := range visitedURLs {
		if u == url {
			// Move to top (end of slice in this logic, but displayed reversed)
			visitedURLs = append(visitedURLs[:i], visitedURLs[i+1:]...)
			visitedURLs = append(visitedURLs, url)
			return
		}
	}

	visitedURLs = append(visitedURLs, url)
	if len(visitedURLs) > 10 {
		visitedURLs = visitedURLs[1:] // Keep last 10
	}
}

func getVisited() []string {
	mu.Lock()
	defer mu.Unlock()
	// Return a copy to avoid race conditions during read/render
	copied := make([]string, len(visitedURLs))
	copy(copied, visitedURLs)
	// We want to display newest first, so let's reverse the copy or handle in template
	// Let's reverse it here for the view
	for i, j := 0, len(copied)-1; i < j; i, j = i+1, j-1 {
		copied[i], copied[j] = copied[j], copied[i]
	}
	return copied
}

func scrapeWebsite(url string, selector string) ([]ScrapeResult, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("status code error: %d %s", res.StatusCode, res.Status)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, err
	}

	var results []ScrapeResult
	doc.Find(selector).Each(func(i int, s *goquery.Selection) {
		title := strings.TrimSpace(s.Text())
		if title == "" {
			return
		}
		
		link, _ := s.Attr("href")
		// Normalize URL
		if link != "" && !strings.HasPrefix(link, "http") && !strings.HasPrefix(link, "mailto:") {
			if strings.HasPrefix(link, "/") {
				// Get root domain
				// Simple heuristic: trim path from url
				// Better: use url.Parse, but for now stick to simple logic or improve.
				// Since 'url' input is full url, we should find the base.
				// For simplicity, let's just prepend the user provided URL (trimmed)
				// Or actually, `goquery` doesn't resolve relative URLs automatically.
				// Let's do a basic fix:
				
				// Find base URL (scheme + host)
				// e.g. https://example.com/foo -> https://example.com
				parts := strings.Split(url, "/")
				if len(parts) >= 3 {
					baseURL := strings.Join(parts[:3], "/")
					link = baseURL + link
				} else {
					// Fallback
					link = strings.TrimSuffix(url, "/") + link
				}
			} else {
				link = fmt.Sprintf("%s/%s", strings.TrimSuffix(url, "/"), link)
			}
		}
		
		results = append(results, ScrapeResult{Title: title, Link: link})
	})

	return results, nil
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Initialize page data
		data := PageData{
			Recommended: recommendedSites,
			Visited:     getVisited(),
		}

		url := r.URL.Query().Get("url")
		selector := r.URL.Query().Get("selector")

		if url != "" {
			data.URL = url
			data.Selector = selector
			addToVisited(url)

			// Default selector logic
			if selector == "" {
				for _, site := range recommendedSites {
					if site.URL == url {
						data.Selector = site.Selector
						selector = site.Selector
						break
					}
				}
			}

			if selector != "" {
				start := time.Now()
				results, err := scrapeWebsite(url, selector)
				data.Duration = time.Since(start).Round(time.Millisecond)
				
				if err != nil {
					data.Error = fmt.Sprintf("Error scraping: %v", err)
				} else {
					data.Results = results
					if len(results) == 0 {
						// Don't set error, just empty results is handled by template
					}
				}
			} else {
				data.Error = "Please provide a CSS selector."
			}
		}

		// Re-parse template in dev mode to see changes without restart? 
		// For production/performance keep the init() one. 
		// For this user script, let's rely on the init() one but add a fallback if it fails or maybe just use it.
		// If the user modifies html, they need to restart go run. That's standard.
		
		if err := tmpl.Execute(w, data); err != nil {
			log.Printf("Template execution error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	})

	fmt.Println("Web Scraper running at http://localhost:8080")
	fmt.Println("Press Ctrl+C to stop")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
