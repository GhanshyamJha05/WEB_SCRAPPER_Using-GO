package main

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

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

var (
	visitedURLs      []string
	mu               sync.Mutex
	recommendedSites = []ScrapingSite{
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
	currentResults  []ScrapeResult
	currentURL      string
	currentSelector string
	darkMode        = false
	scrapeDuration  time.Duration
	resultCount     int
)

func addToVisited(url string) {
	mu.Lock()
	defer mu.Unlock()

	for _, u := range visitedURLs {
		if u == url {
			return
		}
	}

	visitedURLs = append(visitedURLs, url)
	if len(visitedURLs) > 10 {
		visitedURLs = visitedURLs[1:]
	}
}

func scrapeWebsite(url string, selector string) ([]ScrapeResult, error) {
	startTime := time.Now()

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
		link, _ := s.Attr("href")
		if !strings.HasPrefix(link, "http") {
			if strings.HasPrefix(link, "/") {
				link = fmt.Sprintf("%s%s", strings.TrimSuffix(url, "/"), link)
			} else {
				link = fmt.Sprintf("%s/%s", strings.TrimSuffix(url, "/"), link)
			}
		}
		results = append(results, ScrapeResult{Title: title, Link: link})
	})

	scrapeDuration = time.Since(startTime)
	resultCount = len(results)
	return results, nil
}

func renderPage(w http.ResponseWriter, r *http.Request) {
	themeClass := ""
	if darkMode {
		themeClass = "dark-theme"
	}

	fmt.Fprintf(w, `
	<!DOCTYPE html>
	<html>
	<head>
		<title>Web Scraper</title>
		<style>
			:root {
				--bg-color: #f5f5f5;
				--text-color: #333;
				--card-bg: white;
				--border-color: #ddd;
				--primary-color: #4CAF50;
				--secondary-color: #0066cc;
				--hover-color: #e0e0e0;
				--input-bg: white;
			}
			
			.dark-theme {
				--bg-color: #1a1a1a;
				--text-color: #f0f0f0;
				--card-bg: #2d2d2d;
				--border-color: #444;
				--primary-color: #2E7D32;
				--secondary-color: #64B5F6;
				--hover-color: #333;
				--input-bg: #333;
			}
			
			body {
				font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
				max-width: 1200px;
				margin: 0 auto;
				padding: 20px;
				background-color: var(--bg-color);
				color: var(--text-color);
				transition: all 0.3s ease;
			}
			
			.header {
				display: flex;
				justify-content: space-between;
				align-items: center;
				margin-bottom: 20px;
			}
			
			.theme-toggle {
				padding: 8px 15px;
				background-color: var(--primary-color);
				color: white;
				border: none;
				border-radius: 4px;
				cursor: pointer;
				font-weight: bold;
				transition: background-color 0.2s;
				display: flex;
				align-items: center;
				gap: 8px;
			}
			
			.theme-toggle:hover {
				background-color: #3e8e41;
			}
			
			.container {
				display: grid;
				grid-template-columns: 320px 1fr;
				gap: 20px;
			}
			
			.sidebar {
				background-color: var(--card-bg);
				padding: 20px;
				border-radius: 8px;
				box-shadow: 0 2px 10px rgba(0,0,0,0.1);
			}
			
			.results {
				padding: 20px;
				background-color: var(--card-bg);
				border-radius: 8px;
				box-shadow: 0 2px 10px rgba(0,0,0,0.1);
			}
			
			input[type="text"] {
				padding: 10px;
				width: 100%%;
				margin-bottom: 15px;
				border: 1px solid var(--border-color);
				border-radius: 4px;
				background-color: var(--input-bg);
				color: var(--text-color);
			}
			
			button {
				padding: 10px 15px;
				background-color: var(--primary-color);
				color: white;
				border: none;
				border-radius: 4px;
				cursor: pointer;
				width: 100%%;
				font-weight: bold;
				transition: background-color 0.2s;
			}
			
			button:hover {
				background-color: #3e8e41;
			}
			
			.site-card {
				background-color: var(--bg-color);
				padding: 15px;
				margin: 15px 0;
				border-radius: 6px;
				transition: transform 0.2s;
			}
			
			.site-card:hover {
				transform: translateY(-2px);
			}
			
			.tag {
				display: inline-block;
				background-color: var(--primary-color);
				color: white;
				padding: 3px 8px;
				border-radius: 12px;
				font-size: 0.75em;
				margin-left: 8px;
			}
			
			.result-item {
				padding: 12px;
				margin: 8px 0;
				border-bottom: 1px solid var(--border-color);
				transition: background-color 0.2s;
			}
			
			.result-item:hover {
				background-color: var(--hover-color);
			}
			
			.result-item a {
				color: var(--secondary-color);
				text-decoration: none;
			}
			
			.result-item a:hover {
				text-decoration: underline;
			}
			
			h1, h2 {
				margin-top: 0;
			}
			
			.status {
				color: var(--text-color);
				font-style: italic;
				margin: 15px 0;
				padding: 10px;
				background-color: var(--hover-color);
				border-radius: 4px;
			}
			
			.stats {
				display: flex;
				justify-content: space-between;
				margin-bottom: 15px;
				font-size: 0.9em;
				color: var(--text-color);
			}
			
			.copy-btn {
				background-color: var(--secondary-color);
				padding: 5px 10px;
				font-size: 0.8em;
				width: auto;
				margin-left: 10px;
			}
			
			.highlight {
				background-color: rgba(255, 255, 0, 0.3);
				padding: 2px;
			}
		</style>
	</head>
	<body class="%s">
		<div class="header">
			<h1>Web Scraper</h1>
			<button class="theme-toggle" onclick="toggleTheme()">
				<span class="theme-icon">%s</span>
				<span class="theme-text">%s</span>
			</button>
		</div>
		
		<div class="container">
			<div class="sidebar">
				<h2>Scrape a Website</h2>
				<form method="GET" action="/">
					<input type="text" name="url" placeholder="Enter URL" value="%s" required>
					<input type="text" name="selector" placeholder="CSS Selector" value="%s">
					<button type="submit">Scrape</button>
				</form>

				<h2>Recommended Sites</h2>
				<div class="sites-list">
	`, themeClass, getThemeIcon(), getThemeText(), currentURL, currentSelector)

	// Display recommended sites
	for _, site := range recommendedSites {
		fmt.Fprintf(w, `
			<div class="site-card">
				<strong>%s</strong> <span class="tag">%s</span>
				<p>%s</p>
				<p><small>Selector: <code class="highlight">%s</code></small></p>
				<a href="/?url=%s&selector=%s"><button class="copy-btn">Scrape This</button></a>
			</div>
		`, site.URL, site.Tag, site.Example, site.Selector, site.URL, site.Selector)
	}

	fmt.Fprintf(w, `
				</div>

				<h2>Recently Visited</h2>
				<ul>
	`)

	// Display visited URLs (newest first)
	for i := len(visitedURLs) - 1; i >= 0; i-- {
		fmt.Fprintf(w, `<li><a href="/?url=%s">%s</a></li>`, visitedURLs[i], visitedURLs[i])
	}

	fmt.Fprintf(w, `
				</ul>
			</div>
			<div class="results">
				<h2>Scraping Results</h2>
	`)

	if currentURL != "" {
		fmt.Fprintf(w, `
			<div class="status">
				<div class="stats">
					<span>URL: %s</span>
					<span>Results: %d</span>
					<span>Time: %v</span>
				</div>
				<div class="stats">
					<span>Selector: <code class="highlight">%s</code></span>
					<button class="copy-btn" onclick="copyToClipboard('%s')">Copy Selector</button>
				</div>
			</div>
		`, currentURL, resultCount, scrapeDuration.Round(time.Millisecond), currentSelector, currentSelector)
	}

	// Show scraping results if available
	if len(currentResults) > 0 {
		for _, result := range currentResults {
			fmt.Fprintf(w, `
				<div class="result-item">
					<a href="%s" target="_blank">%s</a>
				</div>
			`, result.Link, result.Title)
		}
	} else if currentURL != "" {
		fmt.Fprint(w, `<div class="status">No results found for this URL and selector.</div>`)
	} else {
		fmt.Fprint(w, `<div class="status">Enter a URL and click "Scrape" to see results.</div>`)
	}

	fmt.Fprintf(w, `
			</div>
		</div>
		
		<script>
			function toggleTheme() {
				document.body.classList.toggle('dark-theme');
				const icon = document.querySelector('.theme-icon');
				const text = document.querySelector('.theme-text');
				
				if (document.body.classList.contains('dark-theme')) {
					icon.textContent = '☀️';
					text.textContent = 'Light Mode';
				} else {
					icon.textContent = '🌙';
					text.textContent = 'Dark Mode';
				}
			}
			
			function copyToClipboard(text) {
				navigator.clipboard.writeText(text)
					.then(() => alert('Selector copied to clipboard!'))
					.catch(err => console.error('Could not copy text: ', err));
			}
		</script>
	</body>
	</html>
	`)
}

func getThemeIcon() string {
	if darkMode {
		return "☀️"
	}
	return "🌙"
}

func getThemeText() string {
	if darkMode {
		return "Light Mode"
	}
	return "Dark Mode"
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Check for theme toggle
		if r.URL.Query().Get("theme") == "toggle" {
			darkMode = !darkMode
		}

		url := r.URL.Query().Get("url")
		selector := r.URL.Query().Get("selector")

		if url != "" {
			currentURL = url
			currentSelector = selector
			addToVisited(url)

			// Use default selector if not provided
			if selector == "" {
				for _, site := range recommendedSites {
					if site.URL == url {
						currentSelector = site.Selector
						break
					}
				}
			}

			if currentSelector != "" {
				results, err := scrapeWebsite(url, currentSelector)
				if err != nil {
					fmt.Fprintf(w, "Error scraping website: %v", err)
					return
				}
				currentResults = results
			}
		}

		renderPage(w, r)
	})

	fmt.Println("Web Scraper running at http://localhost:8080")
	fmt.Println("Press Ctrl+C to stop")
	http.ListenAndServe(":8080", nil)
}
