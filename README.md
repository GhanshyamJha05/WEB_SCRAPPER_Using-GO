# 🕸️ Web Scraper Using GO

A concurrent web scraper built with Go. Extract content from any website using CSS selectors — through a web UI, a CLI tool, or a REST API.

<img width="930" height="786" alt="Web Scraper UI" src="https://github.com/user-attachments/assets/499bf87c-6045-49b7-8bcd-997730185caa" />

---

## Features

- **Concurrent scraping** — worker pool drains a shared jobs channel; configurable goroutine count
- **Rate limiting** — global req/s cap shared across all workers, prevents hammering targets
- **Retry + backoff** — failed requests retry up to 3× with exponential backoff
- **Web UI** — responsive dashboard with dark/light mode, history, and recommended sites
- **CLI** — `goscraper` with `--input`, `--selector`, `--workers`, `--output` flags
- **JSON output** — structured envelope with metadata (timestamp, selector, counts, errors)
- **REST API** — `POST /api/bulk-scrape` for programmatic use
- **Vercel deploy** — serverless-ready via `api/index.go`

---

## Project Structure

```
.
├── api/
│   ├── index.go              # Vercel serverless entrypoint (self-contained, no internal/ imports)
│   └── templates/index.html  # UI template (embedded at build time)
├── cmd/goscraper/
│   └── main.go               # CLI entrypoint — flags → pkg/scraper
├── internal/server/
│   └── server.go             # HTTP handler, routing, visited-URL state (standalone server only)
├── pkg/scraper/
│   ├── scraper.go            # Client, Config, ScrapeWithWorkerPool, RunBulkScrape
│   ├── output.go             # ScrapeOutput, ScrapeMeta, SaveJSON
│   ├── pool.go               # Worker pool (goroutines + channels)
│   ├── ratelimiter.go        # Rate limiter (time.Ticker, req/s)
│   └── retry.go              # Exponential backoff retry
├── main.go                   # Standalone HTTP server (~30 lines)
├── CLI_USAGE.md              # Full CLI flag reference
├── Dockerfile
└── vercel.json
```

---

## Quick Start

### Prerequisites

- Go 1.23+
- (Optional) Docker

### Clone and install

```bash
git clone https://github.com/GhanshyamJha05/WEB_SCRAPPER_Using-GO.git
cd WEB_SCRAPPER_Using-GO
go mod tidy
```

### Run the web server

```bash
go run main.go
# → http://localhost:8080
```

### Run the CLI

```bash
# Build
go build -o goscraper ./cmd/goscraper

# Scrape and print to stdout
./goscraper --input urls.txt --selector ".titleline > a"

# Save to file with custom concurrency
./goscraper --input urls.txt --selector ".titleline > a" --workers 8 --output results.json
```

> Full flag reference: [CLI_USAGE.md](./CLI_USAGE.md)

### Run with Docker

```bash
docker build -t web-scraper .
docker run --rm -p 8080:8080 web-scraper
```

---

## CLI Flags

| Flag | Required | Default | Description |
|---|---|---|---|
| `--input` | yes | — | File with one URL per line (`#` = comment) |
| `--selector` | yes | — | CSS selector to extract |
| `--workers` | no | `6` | Concurrent goroutines |
| `--output` | no | stdout | Write JSON to this file |

---

## Output Format

The CLI writes a structured JSON envelope — not a bare array — so the file is self-describing.

```json
{
  "meta": {
    "scraped_at": "2026-04-17T12:01:29Z",
    "selector": ".titleline > a",
    "urls": ["https://news.ycombinator.com"],
    "total_urls": 1,
    "total_items": 30,
    "total_errors": 0,
    "workers": 4
  },
  "results": [
    {
      "title": "Show HN: A new web scraper in Go",
      "link": "https://news.ycombinator.com/item?id=123456"
    }
  ]
}
```

---

## REST API

### `POST /api/bulk-scrape`

**Request**
```json
{
  "urls": ["https://news.ycombinator.com", "https://github.com/trending"],
  "selector": "h2 a"
}
```

**Response**
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

## Configuration

```go
cli := scraper.NewClient(scraper.Config{
    WorkerCount:       6,
    RateLimit:         5.0,                    // max req/s across all workers
    MaxURLsPerRequest: 25,
    HTTPTimeout:       12 * time.Second,
    MaxRetries:        3,
    BaseRetryDelay:    300 * time.Millisecond, // doubles each attempt
})
```

| Field | Default | Description |
|---|---|---|
| `WorkerCount` | `6` | Goroutines in the worker pool |
| `RateLimit` | `5.0` | Max requests per second (global) |
| `MaxURLsPerRequest` | `25` | URL cap per scrape call |
| `HTTPTimeout` | `12s` | Per-request HTTP timeout |
| `MaxRetries` | `3` | Max retry attempts on failure |
| `BaseRetryDelay` | `300ms` | Initial backoff (doubles each attempt) |

---

## Concurrency Design

```
RateLimit = 5.0  →  tick every 200ms

tick   tick   tick   tick   tick
  |      |      |      |      |
 w1     w3     w2     w1     w4    (workers compete for the next tick)
```

Workers pull jobs from a shared channel. Each worker waits for a rate-limiter tick before making a request — global throughput is capped at `RateLimit` req/s regardless of worker count.

---

## Retry Logic

```
attempt 1  fails  →  wait 300ms
attempt 2  fails  →  wait 600ms
attempt 3  fails  →  wait 1200ms
attempt 4  fails  →  return error
```

| Condition | Retried? |
|---|---|
| Network error / timeout | yes |
| HTTP 429 Too Many Requests | yes (honours `Retry-After` header) |
| HTTP 5xx server error | yes |
| HTTP 4xx (except 429) | no |
| HTTP 200 OK | no |

---

## Recommended Selectors

| Site | CSS Selector | Extracts |
|---|---|---|
| Hacker News | `.titleline > a` | Story headlines |
| GitHub Trending | `h2 a` | Repository names |
| Reddit Golang | `h3` | Post titles |

---

## Built With

- [Go](https://golang.org/) — backend, CLI, concurrency
- [goquery](https://github.com/PuerkitoBio/goquery) — CSS selector parsing
- Vanilla HTML/CSS — frontend UI

---

## License

MIT License — © 2025 [Ghanshyam Jha](https://github.com/GhanshyamJha05)
