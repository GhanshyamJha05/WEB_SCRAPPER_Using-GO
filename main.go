package main

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/GhanshyamJha05/WEB_SCRAPPER_Using-GO/internal/server"
	"github.com/GhanshyamJha05/WEB_SCRAPPER_Using-GO/pkg/scraper"
)

// version is set at link time via -ldflags.
var version = "dev"

//go:embed api/templates/index.html
var templateFS embed.FS

func main() {
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}
	tmpl, err := template.New("index.html").Funcs(funcMap).ParseFS(templateFS, "api/templates/index.html")
	if err != nil {
		log.Fatalf("failed to parse template: %v", err)
	}

	cli := scraper.NewClient(scraper.DefaultConfig())
	h := server.New(tmpl, cli)

	http.Handle("/", h)

	fmt.Printf("Web Scraper %s - http://localhost:8080\n", version)
	fmt.Println("Press Ctrl+C to stop")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
