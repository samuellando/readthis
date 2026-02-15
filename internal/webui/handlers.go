// Package webui provides server-rendered HTML views for ReadThis.
package webui

import (
	"bytes"
	"embed"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"readthis/internal/storage"
)

// pageData is the template data for list pages.
type pageData struct {
	IsArchive      bool
	Links          []linkView
	OrderLabel     string
	ToggleOrderURL string
}

// linkView is the template-ready representation of a link.
type linkView struct {
	ID       string
	URL      string
	DaysAgo  string
	Archived bool
}

// templatesFS embeds the HTML templates.
//
//go:embed templates/*.html
var templatesFS embed.FS

// UI serves HTML views backed by the storage layer.
type UI struct {
	store *storage.FileStore
	tpl   *template.Template
	now   func() time.Time
}

// New constructs the UI with a template set and clock function.
func New(store *storage.FileStore, now func() time.Time) (*UI, error) {
	tpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}

	return &UI{store: store, tpl: tpl, now: now}, nil
}

// Handler returns the HTTP mux for UI routes.
func (ui *UI) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", ui.handleActive)
	mux.HandleFunc("/archive", ui.handleArchive)
	mux.HandleFunc("/links", ui.handleCreate)
	mux.HandleFunc("/links/", ui.handleAction)
	return mux
}

// handleActive serves the active links page.
func (ui *UI) handleActive(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeNotFound(w)
		return
	}
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	ui.renderList(w, r, false)
}

// handleArchive serves the archived links page.
func (ui *UI) handleArchive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	ui.renderList(w, r, true)
}

// handleCreate processes the add link form.
func (ui *UI) handleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	if err := r.ParseForm(); err != nil {
		log.Println(err)
		writeBadRequest(w, "invalid form")
		return
	}
	url := strings.TrimSpace(r.FormValue("url"))
	if url == "" {
		writeBadRequest(w, "url is required")
		return
	}

	if _, err := ui.store.Create(url, ui.now().UTC()); err != nil {
		log.Println(err)
		writeServerError(w, "failed to save link")
		return
	}

	redirectBack(w, r, "/")
}

// handleAction archives or unarchives a link.
func (ui *UI) handleAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	clean := strings.TrimPrefix(r.URL.Path, "/links/")
	clean = strings.Trim(clean, "/")
	parts := strings.Split(clean, "/")
	if len(parts) != 2 {
		writeNotFound(w)
		return
	}
	linkID := parts[0]
	action := parts[1]

	if action != "archive" && action != "unarchive" {
		writeNotFound(w)
		return
	}

	var err error
	if action == "archive" {
		_, err = ui.store.Archive(linkID)
	} else {
		_, err = ui.store.Unarchive(linkID)
	}
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeNotFound(w)
			return
		}
		log.Println(err)
		writeServerError(w, "failed to update link")
		return
	}

	if isHTMX(r) {
		w.WriteHeader(http.StatusOK)
		return
	}

	redirectBack(w, r, "/")
}

// renderList renders the active or archive list view.
func (ui *UI) renderList(w http.ResponseWriter, r *http.Request, archived bool) {
	order := storage.OrderNewestFirst
	orderParam := r.URL.Query().Get("order")
	if orderParam == "oldest" {
		order = storage.OrderOldestFirst
	}

	links, err := ui.store.List(order, archived)
	if err != nil {
		log.Println(err)
		writeServerError(w, "failed to load links")
		return
	}

	view := make([]linkView, 0, len(links))
	for _, link := range links {
		view = append(view, linkView{
			ID:       link.ID,
			URL:      link.URL,
			Archived: link.Archived,
			DaysAgo:  formatDaysAgo(ui.now().UTC(), link.CreatedAt),
		})
	}

	label := "newest"
	toggle := "oldest"
	if order == storage.OrderOldestFirst {
		label = "oldest"
		toggle = "newest"
	}

	data := pageData{
		IsArchive:      archived,
		Links:          view,
		OrderLabel:     label,
		ToggleOrderURL: buildOrderURL(archived, toggle),
	}

	var buf bytes.Buffer
	if err := ui.tpl.ExecuteTemplate(&buf, "layout", data); err != nil {
		log.Println(err)
		writeServerError(w, "failed to render page")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

// formatDaysAgo turns timestamps into human-friendly day labels.
func formatDaysAgo(now time.Time, created time.Time) string {
	if created.After(now) {
		created = now
	}
	days := int(now.Sub(created).Hours() / 24)
	if days <= 0 {
		return "today"
	}
	if days == 1 {
		return "1 day ago"
	}
	return "" + strconv.Itoa(days) + " days ago"
}

// buildOrderURL builds the URL for the order toggle link.
func buildOrderURL(archived bool, order string) string {
	base := "/"
	if archived {
		base = "/archive"
	}
	return base + "?order=" + order
}

// isHTMX reports whether the request originated from HTMX.
func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// redirectBack sends a redirect to the referrer when safe.
func redirectBack(w http.ResponseWriter, r *http.Request, fallback string) {
	referer := r.Referer()
	if referer == "" {
		http.Redirect(w, r, fallback, http.StatusSeeOther)
		return
	}
	parsed, err := url.Parse(referer)
	if err != nil || parsed.IsAbs() {
		http.Redirect(w, r, fallback, http.StatusSeeOther)
		return
	}
	redirect := parsed.Path
	if parsed.RawQuery != "" {
		redirect = redirect + "?" + parsed.RawQuery
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

// writeMethodNotAllowed responds with a 405 status.
func writeMethodNotAllowed(w http.ResponseWriter) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

// writeNotFound responds with a 404 status.
func writeNotFound(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
}

// writeBadRequest responds with a 400 status and message.
func writeBadRequest(w http.ResponseWriter, message string) {
	http.Error(w, message, http.StatusBadRequest)
}

// writeServerError responds with a 500 status and message.
func writeServerError(w http.ResponseWriter, message string) {
	http.Error(w, message, http.StatusInternalServerError)
}
