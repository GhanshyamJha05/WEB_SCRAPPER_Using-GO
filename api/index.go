package handler

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

//go:embed templates/index.html
var templateFS embed.FS

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

	// Parse templates from the embedded filesystem
	var err error
	tmpl, err = template.New("index.html").Funcs(funcMap).ParseFS(templateFS, "templates/index.html")
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
		if link != "" && !strings.HasPrefix(link, "http") && !strings.HasPrefix(link, "mailto:") {
			if strings.HasPrefix(link, "/") {
				parts := strings.Split(url, "/")
				if len(parts) >= 3 {
					baseURL := strings.Join(parts[:3], "/")
					link = baseURL + link
				} else {
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

// Handler is the entry point for Vercel
func Handler(w http.ResponseWriter, r *http.Request) {
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
			}
		} else {
			data.Error = "Please provide a CSS selector."
		}
	}

	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
