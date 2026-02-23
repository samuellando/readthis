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
	"sort"
	"strconv"
	"strings"
	"time"

	"readthis/internal/storage"
	"readthis/internal/title"
)

// pageData is the template data for list pages.
type pageData struct {
	IsArchive      bool
	Links          []linkView
	OrderLabel     string
	ToggleOrderURL string
	Count          int
	AllTags        []string
	SelectedTags   []string
	FilterURL      string
}

// linkView is the template-ready representation of a link.
type linkView struct {
	ID               string
	URL              string
	Title            string
	DaysAgo          string
	Archived         bool
	RevisitAt        *time.Time
	DaysUntilRevisit string
	Tags             []string
	TagsDisplay      string
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
	funcMap := template.FuncMap{
		"contains": func(slice []string, item string) bool {
			for _, s := range slice {
				if s == item {
					return true
				}
			}
			return false
		},
	}
	tpl, err := template.New("").Funcs(funcMap).ParseFS(templatesFS, "templates/*.html")
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

	linkTitle := storage.MissingTitle
	if extracted, err := title.Fetch(r.Context(), url); err != nil {
		log.Println(err)
	} else {
		linkTitle = extracted
	}

	if _, err := ui.store.Create(url, linkTitle, ui.now().UTC(), nil); err != nil {
		log.Println(err)
		writeServerError(w, "failed to save link")
		return
	}

	redirectBack(w, r, "/")
}

// handleAction archives or unarchives a link.
func (ui *UI) handleAction(w http.ResponseWriter, r *http.Request) {
	clean := strings.TrimPrefix(r.URL.Path, "/links/")
	clean = strings.Trim(clean, "/")
	parts := strings.Split(clean, "/")
	if len(parts) != 2 {
		writeNotFound(w)
		return
	}
	linkID := parts[0]
	action := parts[1]

	if r.Method == http.MethodGet && action == "title" {
		link, err := ui.store.Get(linkID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				writeNotFound(w)
				return
			}
			log.Println(err)
			writeServerError(w, "failed to load link")
			return
		}
		mode := r.URL.Query().Get("mode")
		ui.renderItem(w, link, mode != "view")
		return
	}

	if r.Method == http.MethodGet && action == "tags" {
		link, err := ui.store.Get(linkID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				writeNotFound(w)
				return
			}
			log.Println(err)
			writeServerError(w, "failed to load link")
			return
		}
		ui.renderItemTagsEdit(w, link)
		return
	}

	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	var updated storage.Link
	var err error
	switch action {
	case "archive":
		updated, err = ui.store.Archive(linkID)
	case "unarchive":
		updated, err = ui.store.Unarchive(linkID)
	case "delete":
		err = ui.store.Delete(linkID)
	case "title":
		if err := r.ParseForm(); err != nil {
			log.Println(err)
			writeBadRequest(w, "invalid form")
			return
		}
		newTitle := strings.TrimSpace(r.FormValue("title"))
		updated, err = ui.store.UpdateTitle(linkID, newTitle)
	case "tags":
		if err := r.ParseForm(); err != nil {
			log.Println(err)
			writeBadRequest(w, "invalid form")
			return
		}
		tagsInput := strings.TrimSpace(r.FormValue("tags"))
		tags := parseTagsInput(tagsInput)
		updated, err = ui.store.SetTags(linkID, tags)
	case "revisit":
		if err := r.ParseForm(); err != nil {
			log.Println(err)
			writeBadRequest(w, "invalid form")
			return
		}
		duration := r.FormValue("duration")
		revisitAt, err := parseDuration(duration, ui.now)
		if err != nil {
			writeBadRequest(w, "invalid duration")
			return
		}
		updated, err = ui.store.SetRevisitAt(linkID, revisitAt)
	default:
		writeNotFound(w)
		return
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
		if action == "title" {
			ui.renderItem(w, updated, false)
			return
		}
		if action == "tags" {
			ui.renderItem(w, updated, false)
			return
		}
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

	var selectedTags []string
	if tagsParam := r.URL.Query().Get("tags"); tagsParam != "" {
		selectedTags = strings.Split(tagsParam, ",")
	}
	if tagParams := r.URL.Query()["tag"]; len(tagParams) > 0 {
		selectedTags = tagParams
	}

	links, err := ui.store.List(order, archived, selectedTags)
	if err != nil {
		log.Println(err)
		writeServerError(w, "failed to load links")
		return
	}

	allTags := make(map[string]bool)
	view := make([]linkView, 0, len(links))
	for _, link := range links {
		v := linkView{
			ID:       link.ID,
			URL:      link.URL,
			Title:    link.Title,
			Archived: link.Archived,
			DaysAgo:  formatDaysAgo(ui.now().UTC(), link.CreatedAt),
			Tags:     link.Tags,
		}
		if len(link.Tags) > 0 {
			v.TagsDisplay = strings.Join(link.Tags, ", ")
			for _, t := range link.Tags {
				allTags[t] = true
			}
		}
		if link.RevisitAt != nil {
			v.RevisitAt = link.RevisitAt
			v.DaysUntilRevisit = formatDaysUntil(ui.now().UTC(), *link.RevisitAt)
		}
		view = append(view, v)
	}

	label := "newest"
	toggle := "oldest"
	if order == storage.OrderOldestFirst {
		label = "oldest"
		toggle = "newest"
	}

	sortedTags := make([]string, 0, len(allTags))
	for t := range allTags {
		sortedTags = append(sortedTags, t)
	}
	sort.Strings(sortedTags)

	filterURL := buildFilterURL(archived, order, selectedTags)

	data := pageData{
		IsArchive:      archived,
		Links:          view,
		OrderLabel:     label,
		ToggleOrderURL: buildOrderURLWithTags(archived, toggle, selectedTags),
		Count:          len(view),
		AllTags:        sortedTags,
		SelectedTags:   selectedTags,
		FilterURL:      filterURL,
	}

	var buf bytes.Buffer
	var templateName string
	if isHTMX(r) {
		templateName = "links-list"
	} else {
		templateName = "layout"
	}
	if err := ui.tpl.ExecuteTemplate(&buf, templateName, data); err != nil {
		log.Println(err)
		writeServerError(w, "failed to render page")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

// renderItem renders a single link item in view or edit mode.
func (ui *UI) renderItem(w http.ResponseWriter, link storage.Link, edit bool) {
	now := ui.now().UTC()
	view := linkView{
		ID:       link.ID,
		URL:      link.URL,
		Title:    link.Title,
		Archived: link.Archived,
		DaysAgo:  formatDaysAgo(now, link.CreatedAt),
		Tags:     link.Tags,
	}
	if len(link.Tags) > 0 {
		view.TagsDisplay = strings.Join(link.Tags, ", ")
	}
	if link.RevisitAt != nil {
		view.RevisitAt = link.RevisitAt
		view.DaysUntilRevisit = formatDaysUntil(now, *link.RevisitAt)
	}

	templateName := "item"
	if edit {
		templateName = "item-edit"
	}

	var buf bytes.Buffer
	if err := ui.tpl.ExecuteTemplate(&buf, templateName, view); err != nil {
		log.Println(err)
		writeServerError(w, "failed to render item")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

// renderItemTagsEdit renders the tags edit form for a link.
func (ui *UI) renderItemTagsEdit(w http.ResponseWriter, link storage.Link) {
	now := ui.now().UTC()
	view := linkView{
		ID:       link.ID,
		URL:      link.URL,
		Title:    link.Title,
		Archived: link.Archived,
		DaysAgo:  formatDaysAgo(now, link.CreatedAt),
		Tags:     link.Tags,
	}
	if len(link.Tags) > 0 {
		view.TagsDisplay = strings.Join(link.Tags, ", ")
	}
	if link.RevisitAt != nil {
		view.RevisitAt = link.RevisitAt
		view.DaysUntilRevisit = formatDaysUntil(now, *link.RevisitAt)
	}

	var buf bytes.Buffer
	if err := ui.tpl.ExecuteTemplate(&buf, "item-tags-edit", view); err != nil {
		log.Println(err)
		writeServerError(w, "failed to render item")
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

// parseDuration parses a duration string (day, week, month, year) into a time.Time.
func parseDuration(duration string, now func() time.Time) (time.Time, error) {
	var t time.Time
	switch duration {
	case "day":
		t = now().AddDate(0, 0, 1)
	case "week":
		t = now().AddDate(0, 0, 7)
	case "month":
		t = now().AddDate(0, 1, 0)
	case "year":
		t = now().AddDate(1, 0, 0)
	default:
		return time.Time{}, errors.New("invalid duration")
	}
	return t, nil
}

// formatDaysUntil turns a future timestamp into days until label.
func formatDaysUntil(now time.Time, revisitAt time.Time) string {
	if revisitAt.Before(now) || revisitAt.Equal(now) {
		return "now"
	}
	days := int(revisitAt.Sub(now).Hours() / 24)
	if days == 0 {
		return "less than a day"
	}
	if days == 1 {
		return "1 day"
	}
	return "" + strconv.Itoa(days) + " days"
}

// buildOrderURL builds the URL for the order toggle link.
func buildOrderURL(archived bool, order string) string {
	base := "/"
	if archived {
		base = "/archive"
	}
	return base + "?order=" + order
}

// buildOrderURLWithTags builds the URL for the order toggle link with tags.
func buildOrderURLWithTags(archived bool, order string, tags []string) string {
	base := "/"
	if archived {
		base = "/archive"
	}
	params := url.Values{}
	params.Set("order", order)
	if len(tags) > 0 {
		params.Set("tags", strings.Join(tags, ","))
	}
	return base + "?" + params.Encode()
}

// buildFilterURL builds the URL for the filter action.
func buildFilterURL(archived bool, order storage.Order, tags []string) string {
	base := "/"
	if archived {
		base = "/archive"
	}
	params := url.Values{}
	if order == storage.OrderOldestFirst {
		params.Set("order", "oldest")
	}
	if len(tags) > 0 {
		params.Set("tags", strings.Join(tags, ","))
	}
	if len(params) == 0 {
		return base
	}
	return base + "?" + params.Encode()
}

// parseTagsInput parses a comma-separated tags string into a slice.
func parseTagsInput(input string) []string {
	if input == "" {
		return nil
	}
	parts := strings.Split(input, ",")
	result := make([]string, 0, len(parts))
	seen := make(map[string]bool)
	for _, p := range parts {
		tag := strings.TrimSpace(p)
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		result = append(result, tag)
	}
	if len(result) == 0 {
		return nil
	}
	return result
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
