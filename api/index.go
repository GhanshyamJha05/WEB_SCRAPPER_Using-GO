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

type ScrapeJob struct {
	URL      string
	Selector string
}

type JobResult struct {
	URL      string
	Selector string
	Items    []ScrapeResult
	Err      error
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

const (
	workerCount        = 5
	requestInterval    = 250 * time.Millisecond
	maxURLsPerRequest  = 20
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

func resolveLink(base string, link string) string {
	if link == "" || strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") || strings.HasPrefix(link, "mailto:") {
		return link
	}

	parts := strings.Split(base, "/")
	if len(parts) < 3 {
		return link
	}

	root := strings.Join(parts[:3], "/")
	if strings.HasPrefix(link, "/") {
		return root + link
	}

	return strings.TrimSuffix(base, "/") + "/" + link
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
		link = resolveLink(url, link)

		results = append(results, ScrapeResult{Title: title, Link: link})
	})

	return results, nil
}

func worker(id int, jobs <-chan ScrapeJob, results chan<- JobResult, limiter <-chan time.Time) {
	for job := range jobs {
		<-limiter
		items, err := scrapeWebsite(job.URL, job.Selector)
		results <- JobResult{
			URL:      job.URL,
			Selector: job.Selector,
			Items:    items,
			Err:      err,
		}
	}
}

func parseURLs(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		u := strings.TrimSpace(p)
		if u == "" {
			continue
		}
		out = append(out, u)
	}
	return out
}

func scrapeWithWorkerPool(urls []string, selector string) ([]ScrapeResult, []error) {
	jobs := make(chan ScrapeJob, len(urls))
	results := make(chan JobResult, len(urls))

	limiter := time.NewTicker(requestInterval)
	defer limiter.Stop()

	activeWorkers := workerCount
	if len(urls) < activeWorkers {
		activeWorkers = len(urls)
	}
	if activeWorkers == 0 {
		return nil, nil
	}

	var wg sync.WaitGroup
	for i := 0; i < activeWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			worker(workerID, jobs, results, limiter.C)
		}(i + 1)
	}

	for _, u := range urls {
		jobs <- ScrapeJob{URL: u, Selector: selector}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	combined := make([]ScrapeResult, 0)
	errs := make([]error, 0)
	for result := range results {
		if result.Err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", result.URL, result.Err))
			continue
		}
		combined = append(combined, result.Items...)
	}
	return combined, errs
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
		urls := parseURLs(url)
		if len(urls) == 0 {
			data.Error = "Please provide at least one valid URL."
			if err := tmpl.Execute(w, data); err != nil {
				log.Printf("Template execution error: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
			return
		}
		if len(urls) > maxURLsPerRequest {
			data.Error = fmt.Sprintf("Too many URLs. Maximum allowed per request is %d.", maxURLsPerRequest)
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
			results, errs := scrapeWithWorkerPool(urls, selector)
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
}
