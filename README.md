# 🕸️ Web Scraper in Go

A simple, lightweight web scraper with a modern web UI — built using Go. Scrape content from your favorite websites using CSS selectors and view results in real-time with theme support.

<img width="1847" height="898" alt="image" src="https://github.com/user-attachments/assets/d44f7683-567e-4119-bb19-916c77249290" />

---

## 🔍 Features

- ⚡ Real-time scraping via CSS selectors  
- 🌐 Predefined sites (e.g., Hacker News, Reddit Golang, GitHub Trending)  
- 🌙 Dark/Light theme toggle  
- � Copay selector for quick reuse  
- 🧠 Smart fallback for missing selectors  
- 📜 History of recently scraped URLs  
- 🛠️ Built with Go and `goquery`  

---

## � Insotallation

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

1. **Build the Docker image**
   ```bash
   docker build -t web-scraper .
   ```

2. **Run the container**
   ```bash
   docker run -p 8080:8080 web-scraper
   ```

3. **Access the scraper**
   Open your browser and visit:  
   [http://localhost:8080](http://localhost:8080)

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

## 🚀 Deployment Options

### Deploy to Render
1. Fork this repository
2. Sign up at [Render.com](https://render.com)
3. Create a new Web Service
4. Connect your GitHub repository
5. Use these settings:
   - Build Command: `go build -o web-scraper`
   - Start Command: `./web-scraper`

### Deploy to Railway
1. Sign up at [Railway.app](https://railway.app)
2. Click "New Project" → "Deploy from GitHub repo"
3. Select this repository
4. Railway will auto-detect Go and deploy

### Deploy to Fly.io
```bash
# Install flyctl
curl -L https://fly.io/install.sh | sh

# Login and launch
fly auth login
fly launch
fly deploy
```

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
