// Package web serves Sift's UI: server-rendered html/template pages plus
// htmx for interactivity. No SPA, no separate frontend build step --
// templates and static assets are embedded in the binary via go:embed, so
// a self-hosted deploy is exactly the compiled Sift binary and nothing
// else. html/template (not text/template) is a deliberate choice here:
// pages render subject lines and snippets pulled straight from someone's
// inbox, which is attacker-controlled input the moment a phishing email
// lands in it, and html/template's context-aware auto-escaping is what
// keeps that from becoming stored XSS.
package web

import (
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// pages holds one parsed template set per page, each combining the shared
// layout with that page's own "content" definition. They're parsed
// separately (not all at once) because every page defines a template
// named "content" -- parsing them together into one shared set would let
// each later file silently overwrite the last one's definition.
var pages = map[string]*template.Template{
	"dashboard":          mustParsePage("dashboard.html"),
	"application_detail": mustParsePage("application_detail.html"),
	"review_queue":       mustParsePage("review_queue.html"),
}

func mustParsePage(name string) *template.Template {
	return template.Must(template.ParseFS(templateFS, "templates/layout.html", "templates/"+name))
}

func render(w http.ResponseWriter, page string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pages[page].ExecuteTemplate(w, "layout", data); err != nil {
		slog.Error("render template failed", "page", page, "error", err)
	}
}

// StaticHandler serves the embedded static assets (CSS, vendored htmx) at
// whatever prefix the caller mounts it under.
func StaticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err) // embed.FS with a compile-time-checked path; unreachable
	}
	return http.StripPrefix("/static/", http.FileServerFS(sub))
}
