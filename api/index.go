// Package handler is the Vercel serverless entrypoint.
package handler

import (
	"embed"
	"html/template"
	"log"
	"net/http"

	"github.com/GhanshyamJha05/WEB_SCRAPPER_Using-GO/internal/server"
	"github.com/GhanshyamJha05/WEB_SCRAPPER_Using-GO/pkg/scraper"
)

//go:embed templates/index.html
var templateFS embed.FS

var h *server.Handler

func init() {
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}
	tmpl, err := template.New("index.html").Funcs(funcMap).ParseFS(templateFS, "templates/index.html")
	if err != nil {
		log.Fatalf("failed to parse template: %v", err)
	}
	cli := scraper.NewClient(scraper.DefaultConfig())
	h = server.New(tmpl, cli)
}

// Handler is the Vercel serverless function entrypoint.
func Handler(w http.ResponseWriter, r *http.Request) {
	h.ServeHTTP(w, r)
}
