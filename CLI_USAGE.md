# goscraper CLI Usage

`goscraper` is a concurrent web scraper that reads URLs from a file, applies a CSS selector, and outputs results as JSON.

---

## Build

```bash
go build -o goscraper ./cmd/goscraper
```

On Windows:

```bash
go build -o goscraper.exe ./cmd/goscraper
```

---

## Flags

| Flag | Required | Default | Description |
|---|---|---|---|
| `--input` | yes | — | Path to a text file with one URL per line |
| `--selector` | yes | — | CSS selector used to extract elements |
| `--workers` | no | `6` | Number of concurrent goroutines |
| `--output` | no | stdout | File path to write JSON results to |

---

## Input File Format

One URL per line. Blank lines and lines starting with `#` are ignored.

```
# Hacker News
https://news.ycombinator.com

# GitHub
https://github.com/trending
https://github.com/explore
```

---

## Examples

### Basic — print to stdout

```bash
./goscraper --input urls.txt --selector ".titleline > a"
```

### Save output to a file

```bash
./goscraper --input urls.txt --selector ".titleline > a" --output results.json
```

### Increase concurrency

```bash
./goscraper --input urls.txt --selector "h2 a" --workers 12 --output results.json
```

### Scrape a single URL (one-liner input file)

```bash
echo "https://news.ycombinator.com" > single.txt
./goscraper --input single.txt --selector ".titleline > a"
```

---

## Output Format

Results are written as a JSON array. Each element represents one matched element on the page.

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

- `title` — trimmed inner text of the matched element
- `link` — resolved absolute `href` (empty string if the element has no `href`)

---

## Error Handling

Per-URL errors (network failures, bad status codes, etc.) are printed to **stderr** as warnings and do not stop the run. Successful results from other URLs are still written to the output.

```
warn: https://example.com: dial tcp: connection refused
```

The process exits with code `1` only if a fatal error occurs (missing flags, unreadable input file, unwritable output file).

---

## Common CSS Selectors

| Site | Selector | Extracts |
|---|---|---|
| Hacker News | `.titleline > a` | Story headlines |
| GitHub Trending | `h2 a` | Repository names |
| Reddit | `h3` | Post titles |
| Generic blog | `article h2 a` | Article titles |

---

## Tips

- Use `--workers 1` to scrape sequentially (useful for rate-sensitive sites).
- Pipe stdout to `jq` for quick filtering:
  ```bash
  ./goscraper --input urls.txt --selector ".titleline > a" | jq '.[].title'
  ```
- The built-in rate limiter (default 5 req/s) is shared across all workers, so increasing `--workers` won't hammer a single host beyond the configured rate.
