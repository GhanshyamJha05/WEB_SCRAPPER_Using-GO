# 🕸️ Web Scraper in Go

A simple, lightweight web scraper with a modern web UI — built using Go. Scrape content from your favorite websites using CSS selectors and view results in real-time with theme support.

<img width="100%" alt="GoScraper UI" src="screenshot.png" />

---

## 🔍 Features

- ⚡ Real-time scraping via CSS selectors  
- 🌐 Predefined sites (e.g., Hacker News, Reddit Golang, GitHub Trending)  
- 🌙 Dark/Light theme toggle  
- 📋 Copy selector for quick reuse  
- 🧠 Smart fallback for missing selectors  
- 📜 History of recently scraped URLs  
- 🛠️ Built with Go and `goquery`  
- 🐳 Multi-stage **Docker** image (distroless, non-root)  
- 🖥️ **`goscraper` CLI** with optional **Prometheus** `/metrics`  
- ✅ **GitHub Actions** CI (test, vet, Docker build)  

---

## 📥 Installation

### Option 1: Run Locally

1. **Clone the repository**
   ```bash
   git clone https://github.com/GhanshyamJha05/WEB_SCRAPPER_Using-GO.git
   cd WEB_SCRAPPER_Using-GO
   ```

2. **Install dependencies**
   Make sure you have Go installed. Then:

   ```bash
   go mod tidy
   ```

3. **Run the server**
   ```bash
   go run main.go
   ```

4. **Access the scraper**
   Open your browser and visit:  
   [http://localhost:8080](http://localhost:8080)

### Option 2: Run with Docker

The `Dockerfile` is **multi-stage**: compiles a static binary, then runs it as a **non-root** user on **gcr.io/distroless/static-debian12**.

1. **Build the image**
   ```bash
   docker build -t web-scraper --build-arg VERSION=$(git rev-parse --short HEAD) .
   ```

2. **Run the container**
   ```bash
   docker run --rm -p 8080:8080 web-scraper
   ```

3. Open [http://localhost:8080](http://localhost:8080)

### Option 3: CLI (`goscraper`) + Prometheus metrics

Build the CLI (same module, `cmd/goscraper`):

```bash
go build -o goscraper ./cmd/goscraper
```

**Single URL**

```bash
./goscraper scrape -url https://news.ycombinator.com -selector ".titleline > a"
./goscraper scrape -url https://example.com -selector "a" -json
```

**Bulk (JSON output matches the HTTP bulk API)**

```bash
./goscraper bulk -selector ".titleline > a" https://news.ycombinator.com https://github.com/trending
./goscraper bulk -selector "h2 a" -f urls.txt
```

Expose **Prometheus** metrics and Go/process collectors (optional):

```bash
./goscraper --metrics-listen :9091 bulk -selector ".titleline > a" https://news.ycombinator.com
# curl http://localhost:9091/metrics
# curl http://localhost:9091/healthz
```

Environment variable: `GOSCRAPER_METRICS_LISTEN` (same as `--metrics-listen`).

---

## 🤖 CI/CD

On every push and pull request, GitHub Actions (`.github/workflows/ci.yml`) runs `go vet`, `go test`, builds the server and CLI, and **builds the Docker image** (validation only; image is not pushed by default).

---

## ✨ Example Sites Supported

| Site            | Tag       | Example          | CSS Selector              |
|-----------------|-----------|------------------|---------------------------|
| Hacker News     | Tech News | Headlines        | `.titleline > a`          |
| Reddit Golang   | Golang    | Post titles      | `h3._eYtD2XCVieq6emjKBH3m`|
| GitHub Trending | GitHub    | Trending Repos   | `h2 a`                    |

---

## 🧠 How It Works

1. Enter a URL and (optionally) a CSS selector.
2. Click **Scrape** to fetch and display titles/links.
3. View results styled in a readable layout.
4. Toggle between light/dark themes.
5. Try recommended sites or browse recently scraped URLs.

---

## 🧰 Built With

- [Go](https://golang.org/)
- [goquery](https://github.com/PuerkitoBio/goquery)
- HTML/CSS (embedded via `http.ResponseWriter`)

---



---

## 🚀 Future Ideas

- Export results to JSON/CSV  
- Pagination for large results  
- Login/session-based scraping  
- API endpoint for programmatic access

---

## 📝 License

MIT License  
© 2025 [Ghanshyam Jha](https://github.com/GhanshyamJha05)
