package handler

import (
	"context"
	"embed"
	"encoding/json"
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
	Title string `json:"title"`
	Link  string `json:"link"`
}

type ScrapeJob struct {
	Index    int
	URL      string
	Selector string
}

type JobResult struct {
	Index      int
	URL        string
	Items      []ScrapeResult
	DurationMs int64
	Err        error
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

type BulkScrapeRequest struct {
	URLs     []string `json:"urls"`
	Selector string   `json:"selector"`
}

type BulkScrapeResult struct {
	URL             string `json:"url"`
	Data            string `json:"data"`
	ExecutionTimeMs int64  `json:"execution_time_ms"`
	Status          string `json:"status"`
}

type BulkScrapeResponse struct {
	TotalBatchTimeMs int               `json:"total_batch_time_ms"`
	Results          []BulkScrapeResult `json:"results"`
}

// Global state (Thread-safe)
var (
	visitedURLs []string
	mu          sync.Mutex
	tmpl        *template.Template
	httpClient  = &http.Client{Timeout: 12 * time.Second}
)

const (
	workerCount       = 6
	requestInterval   = 200 * time.Millisecond
	maxURLsPerRequest = 25
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

func scrapeWebsite(ctx context.Context, url string, selector string) ([]ScrapeResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	res, err := httpClient.Do(req)
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
		start := time.Now()
		items, err := scrapeWebsite(context.Background(), job.URL, job.Selector)
		results <- JobResult{
			Index:      job.Index,
			URL:        job.URL,
			Items:      items,
			DurationMs: time.Since(start).Milliseconds(),
			Err:        err,
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

func runBulkScrape(urls []string, selector string) BulkScrapeResponse {
	start := time.Now()
	response := BulkScrapeResponse{
		Results: make([]BulkScrapeResult, len(urls)),
	}

	jobs := make(chan ScrapeJob, len(urls))
	results := make(chan JobResult, len(urls))
	limiter := time.NewTicker(requestInterval)
	defer limiter.Stop()

	activeWorkers := workerCount
	if len(urls) < activeWorkers {
		activeWorkers = len(urls)
	}
	if activeWorkers == 0 {
		return response
	}

	var wg sync.WaitGroup
	for i := 0; i < activeWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			worker(workerID, jobs, results, limiter.C)
		}(i + 1)
	}

	for i, u := range urls {
		jobs <- ScrapeJob{Index: i, URL: u, Selector: selector}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		extracted := make([]string, 0, len(result.Items))
		for _, item := range result.Items {
			if trimmed := strings.TrimSpace(item.Title); trimmed != "" {
				extracted = append(extracted, trimmed)
			}
		}
		row := BulkScrapeResult{
			URL:             result.URL,
			Data:            strings.Join(extracted, " | "),
			ExecutionTimeMs: result.DurationMs,
			Status:          "success",
		}
		if result.Err != nil {
			row.Data = result.Err.Error()
			row.Status = "failed"
		}
		response.Results[result.Index] = row
	}

	response.TotalBatchTimeMs = int(time.Since(start).Milliseconds())
	return response
}

func bulkScrapeAPIHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload BulkScrapeRequest
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
	if len(urls) > maxURLsPerRequest {
		http.Error(w, fmt.Sprintf("Maximum %d URLs allowed per request", maxURLsPerRequest), http.StatusBadRequest)
		return
	}

	for _, u := range urls {
		addToVisited(u)
	}

	response := runBulkScrape(urls, selector)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// Handler is the entry point for Vercel
func Handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/bulk-scrape" || strings.HasSuffix(r.URL.Path, "/bulk-scrape") {
		bulkScrapeAPIHandler(w, r)
		return
	}

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
