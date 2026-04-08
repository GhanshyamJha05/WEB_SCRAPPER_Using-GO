# 🕸️ Web Scraper Using Go

A powerful, lightweight, and concurrent web scraper built with Go. Extract content from any website using CSS selectors through a modern web UI, a robust CLI tool, or a REST API

<img width="930" height="786" alt="image" src="https://github.com/user-attachments/assets/499bf87c-6045-49b7-8bcd-997730185caa" />


---

## 🚀 What It Does

This tool allows you to scrape data from websites efficiently using **Go's concurrency primitives**. Whether you need to extract headlines from news sites, post titles from Reddit, or repository names from GitHub, this scraper makes it easy.

- **Concurrent Scraping**: Uses a worker pool to scrape multiple URLs simultaneously with controllable rate limits.
- **Web Interface**: A sleek, responsive dashboard with dark/light mode support.
- **CLI Power**: A dedicated `goscraper` command for terminal enthusiasts and automation.
- **REST API**: A JSON-based bulk scraping endpoint for programmatic integration.
- **Smart Extracts**: Automatically resolves relative links to absolute URLs.

---

## 🛠️ How to Run

### 1. Prerequisites
- **Go 1.21+** installed on your system.
- (Optional) **Docker** for containerized execution.

### 2. Local Setup
```bash
# Clone the repository
git clone https://github.com/GhanshyamJha05/WEB_SCRAPPER_Using-GO.git
cd WEB_SCRAPPER_Using-GO

# Install dependencies
go mod tidy
```

### 3. Running the Web Server
```bash
go run main.go
```
The server will start at [http://localhost:8080](http://localhost:8080).

### 4. Running the CLI
```bash
# Build the CLI
go build -o goscraper ./cmd/goscraper

# Single URL scrape
./goscraper scrape -url https://news.ycombinator.com -selector ".titleline > a"

# Bulk scrape (JSON output)
./goscraper bulk -selector "h2 a" https://github.com/trending https://news.ycombinator.com
```

### 5. Running with Docker
```bash
docker build -t web-scraper .
docker run --rm -p 8080:8080 web-scraper
```

---

## 📊 Sample Output

### CLI (JSON Format)
```json
[
  {
    "title": "Show HN: A new web scraper in Go",
    "link": "https://news.ycombinator.com/item?id=123456"
  },
  {
    "title": "Why Go is great for scraping",
    "link": "https://example.com/blog/go-scraping"
  }
]
```

### API Bulk Scrape Response
```json
{
  "total_batch_time_ms": 450,
  "results": [
    {
      "url": "https://news.ycombinator.com",
      "data": "Headline 1 | Headline 2 | Headline 3",
      "execution_time_ms": 210,
      "status": "success"
    }
  ]
}
```

---

## What It Does

Scrape data from websites efficiently using Go's concurrency primitives. Whether you need headlines from Hacker News, post titles from Reddit, or trending repos from GitHub — this scraper handles it fast.

- **Worker Pool**: Configurable goroutines drain a shared jobs channel concurrently.
- **Rate Limiting**: Global `RateLimit` (req/s) — one shared ticker across all workers.
- **Retry + Backoff**: Failed requests retry up to 3 times with exponential backoff.
- **Modular Structure**: Clean `cmd/` / `pkg/` / `internal/` layout.
- **Web Interface**: Responsive dashboard with dark/light mode, history, and recommended sites.
- **CLI**: `goscraper` command with single and bulk scrape modes + optional Prometheus metrics.
- **REST API**: JSON bulk scraping endpoint, deployable to Vercel.

---

## Project Structure

```
.
├── cmd/goscraper/       # CLI binary entrypoint (thin main, flags -> pkg/scraper)
├── pkg/scraper/         # Reusable scraping engine (public API)
│   ├── scraper.go       # Client, Config, fetch, ScrapeWithWorkerPool, RunBulkScrape
│   ├── pool.go          # Worker pool primitive (goroutines + channels)
│   ├── ratelimiter.go   # Rate limiter (time.Ticker, configurable req/s)
│   └── retry.go         # Retry logic with exponential backoff
├── internal/server/     # HTTP handler logic (not importable externally)
│   └── server.go        # Handler, PageData, routing, visited URL state
├── api/
│   ├── index.go         # Vercel serverless entrypoint (delegates to internal/server)
│   └── templates/       # HTML template
└── main.go              # Standalone HTTP server entrypoint (~30 lines)
```

- `pkg/` — importable by anyone, including external projects.
- `internal/` — app-specific logic the Go toolchain prevents external packages from importing.
- `cmd/` — binary entrypoints only; stays thin by delegating to `pkg/` and `internal/`.

---

## Concurrency Design

```
RateLimit = 5.0  ->  interval = 1s / 5 = 200ms

tick  tick  tick  tick  tick   (one every 200ms)
  |     |     |     |     |
worker1 worker3 worker2 worker1 worker4
```

Workers compete to pull jobs from an unbuffered channel (natural backpressure). Before each request every worker calls `rl.wait()` on a shared `rateLimiter` — global throughput is capped at `RateLimit` req/s regardless of worker count.

---

## Retry Logic

Failed requests are automatically retried with exponential backoff:

```
attempt 0  fails  ->  wait 300ms
attempt 1  fails  ->  wait 600ms
attempt 2  fails  ->  wait 1200ms
attempt 3  fails  ->  return error
```

| Condition | Retried? |
|---|---|
| Network error / timeout | yes |
| HTTP 429 Too Many Requests | yes (honours `Retry-After` header) |
| HTTP 5xx server error | yes |
| HTTP 4xx (except 429) | no |
| HTTP 200 | no |

Configurable per client via `MaxRetries` and `BaseRetryDelay` in `Config`.

---

## How to Run

### Prerequisites
- Go 1.23+
- (Optional) Docker

### Local Setup
```bash
git clone https://github.com/GhanshyamJha05/WEB_SCRAPPER_Using-GO.git
cd WEB_SCRAPPER_Using-GO
go mod tidy
```

### Web Server
```bash
go run main.go
# -> http://localhost:8080
```

### CLI
```bash
# Build
go build -o goscraper ./cmd/goscraper

# Single URL
./goscraper scrape -url https://news.ycombinator.com -selector ".titleline > a"

# Single URL — JSON output
./goscraper scrape -url https://news.ycombinator.com -selector ".titleline > a" -json

# Bulk scrape
./goscraper bulk -selector "h2 a" https://github.com/trending https://news.ycombinator.com

# Bulk from file
./goscraper bulk -selector ".titleline > a" -f urls.txt

# With Prometheus metrics
./goscraper --metrics-listen :9091 scrape -url https://news.ycombinator.com -selector ".titleline > a"
```

### Docker
```bash
docker build -t web-scraper .
docker run --rm -p 8080:8080 web-scraper
```

---

## Configuration

```go
cli := scraper.NewClient(scraper.Config{
    WorkerCount:       6,
    RateLimit:         5.0,                  // max requests/sec across all workers
    MaxURLsPerRequest: 25,
    HTTPTimeout:       12 * time.Second,
    MaxRetries:        3,                    // retry up to 3 times on failure
    BaseRetryDelay:    300 * time.Millisecond, // 300ms -> 600ms -> 1200ms
})
```

| Field | Default | Description |
|---|---|---|
| `WorkerCount` | `6` | Number of worker goroutines |
| `RateLimit` | `5.0` | Max requests per second (global) |
| `MaxURLsPerRequest` | `25` | URL cap per scrape call |
| `HTTPTimeout` | `12s` | Per-request HTTP timeout |
| `MaxRetries` | `3` | Max retry attempts on failure |
| `BaseRetryDelay` | `300ms` | Initial backoff delay (doubles each attempt) |

---

## Sample Output

### CLI (JSON)
```json
[
  { "title": "Show HN: A new web scraper in Go", "link": "https://news.ycombinator.com/item?id=123456" },
  { "title": "Why Go is great for scraping",     "link": "https://example.com/blog/go-scraping" }
]
```

### Bulk API (`POST /api/bulk-scrape`)
```json
{
  "total_batch_time_ms": 450,
  "results": [
    {
      "url": "https://news.ycombinator.com",
      "data": "Headline 1 | Headline 2 | Headline 3",
      "execution_time_ms": 210,
      "status": "success"
    }
  ]
}
```

---

## Recommended Sites

| Site | Tag | CSS Selector |
|---|---|---|
| Hacker News | Tech News | `.titleline > a` |
| Reddit Golang | Golang | `h3._eYtD2XCVieq6emjKBH3m` |
| GitHub Trending | GitHub | `h2 a` |

---

## Built With

- [Go](https://golang.org/) — backend, CLI, concurrency
- [goquery](https://github.com/PuerkitoBio/goquery) — CSS selector parsing
- [Prometheus](https://prometheus.io/) — CLI metrics (`/metrics`, `/healthz`)
- Vanilla HTML/CSS — frontend UI

---

## License

MIT License — © 2025 [Ghanshyam Jha](https://github.com/GhanshyamJha05)
