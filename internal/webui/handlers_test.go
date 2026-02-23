package webui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"readthis/internal/storage"
)

func readBody(resp *http.Response) (string, error) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func TestCreateLink(t *testing.T) {
	// Ensures creating a link stores the title.
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><head><title>Created Page</title></head><body>Hello</body></html>"))
	}))
	defer page.Close()

	now := time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC)
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	ui, err := New(store, func() time.Time { return now })
	if err != nil {
		t.Fatalf("new ui: %v", err)
	}

	server := httptest.NewServer(ui.Handler())
	defer server.Close()

	form := "url=" + page.URL
	resp, err := http.Post(server.URL+"/links", "application/x-www-form-urlencoded", strings.NewReader(form))
	if err != nil {
		t.Fatalf("create link: %v", err)
	}
	resp.Body.Close()

	links, err := store.List(storage.OrderNewestFirst, false, nil)
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].Title != "Created Page" {
		t.Fatalf("expected stored title, got %q", links[0].Title)
	}
}

func TestArchiveLink(t *testing.T) {
	// Ensures archive action updates the link.
	now := time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC)
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	created, err := store.Create("https://example.com/archive", "Archive Title", now, nil)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	ui, err := New(store, func() time.Time { return now })
	if err != nil {
		t.Fatalf("new ui: %v", err)
	}

	server := httptest.NewServer(ui.Handler())
	defer server.Close()

	resp, err := http.Post(server.URL+"/links/"+created.ID+"/archive", "application/x-www-form-urlencoded", nil)
	if err != nil {
		t.Fatalf("archive link: %v", err)
	}
	resp.Body.Close()

	archived, err := store.List(storage.OrderNewestFirst, true, nil)
	if err != nil {
		t.Fatalf("list archived: %v", err)
	}
	if len(archived) != 1 || archived[0].ID != created.ID {
		t.Fatalf("expected archived link")
	}
}

func TestUpdateTitle(t *testing.T) {
	// Ensures title edit updates the stored link.
	now := time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC)
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	created, err := store.Create("https://example.com/title", "Old Title", now, nil)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	ui, err := New(store, func() time.Time { return now })
	if err != nil {
		t.Fatalf("new ui: %v", err)
	}

	server := httptest.NewServer(ui.Handler())
	defer server.Close()

	form := "title=New+Title"
	resp, err := http.Post(server.URL+"/links/"+created.ID+"/title", "application/x-www-form-urlencoded", strings.NewReader(form))
	if err != nil {
		t.Fatalf("update title: %v", err)
	}
	resp.Body.Close()

	links, err := store.List(storage.OrderNewestFirst, false, nil)
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].Title != "New Title" {
		t.Fatalf("expected updated title, got %q", links[0].Title)
	}
}

func TestDeleteLink(t *testing.T) {
	// Ensures delete action removes the link.
	now := time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC)
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	created, err := store.Create("https://example.com/delete", "Delete Title", now, nil)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	ui, err := New(store, func() time.Time { return now })
	if err != nil {
		t.Fatalf("new ui: %v", err)
	}

	server := httptest.NewServer(ui.Handler())
	defer server.Close()

	resp, err := http.Post(server.URL+"/links/"+created.ID+"/delete", "application/x-www-form-urlencoded", nil)
	if err != nil {
		t.Fatalf("delete link: %v", err)
	}
	resp.Body.Close()

	links, err := store.List(storage.OrderNewestFirst, false, nil)
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(links) != 0 {
		t.Fatalf("expected no links, got %d", len(links))
	}
}

func TestEditTags(t *testing.T) {
	// Ensures tag edit updates the stored link.
	now := time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC)
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	created, err := store.Create("https://example.com/tags", "Tag Test", now, nil)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	ui, err := New(store, func() time.Time { return now })
	if err != nil {
		t.Fatalf("new ui: %v", err)
	}

	server := httptest.NewServer(ui.Handler())
	defer server.Close()

	form := "tags=go%2C+tutorial%2C+go"
	resp, err := http.Post(server.URL+"/links/"+created.ID+"/tags", "application/x-www-form-urlencoded", strings.NewReader(form))
	if err != nil {
		t.Fatalf("update tags: %v", err)
	}
	resp.Body.Close()

	links, err := store.List(storage.OrderNewestFirst, false, nil)
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if len(links[0].Tags) != 2 {
		t.Fatalf("expected 2 tags after dedup, got %d: %v", len(links[0].Tags), links[0].Tags)
	}
	if links[0].Tags[0] != "go" || links[0].Tags[1] != "tutorial" {
		t.Fatalf("unexpected tags: %v", links[0].Tags)
	}
}

func TestFilterByTags(t *testing.T) {
	// Ensures filtering by tags works correctly.
	now := time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC)
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	_, _ = store.Create("https://example.com/go", "Go Link", now, []string{"go", "tutorial"})
	_, _ = store.Create("https://example.com/rust", "Rust Link", now, []string{"rust", "tutorial"})
	_, _ = store.Create("https://example.com/other", "Other Link", now, []string{"misc"})

	ui, err := New(store, func() time.Time { return now })
	if err != nil {
		t.Fatalf("new ui: %v", err)
	}

	server := httptest.NewServer(ui.Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/?tag=go")
	if err != nil {
		t.Fatalf("filter by go tag: %v", err)
	}
	body, _ := readBody(resp)
	if !strings.Contains(body, "Go Link") {
		t.Fatalf("expected Go Link in filtered results")
	}
	if strings.Contains(body, "Rust Link") {
		t.Fatalf("expected Rust Link not in filtered results")
	}
	if strings.Contains(body, "Other Link") {
		t.Fatalf("expected Other Link not in filtered results")
	}
}

func TestFilterByMultipleTags(t *testing.T) {
	// Ensures filtering by multiple tags uses OR logic.
	now := time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC)
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	_, _ = store.Create("https://example.com/go", "Go Link", now, []string{"go"})
	_, _ = store.Create("https://example.com/rust", "Rust Link", now, []string{"rust"})
	_, _ = store.Create("https://example.com/other", "Other Link", now, []string{"misc"})

	ui, err := New(store, func() time.Time { return now })
	if err != nil {
		t.Fatalf("new ui: %v", err)
	}

	server := httptest.NewServer(ui.Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/?tag=go&tag=rust")
	if err != nil {
		t.Fatalf("filter by go and rust tags: %v", err)
	}
	body, _ := readBody(resp)
	if !strings.Contains(body, "Go Link") {
		t.Fatalf("expected Go Link in filtered results")
	}
	if !strings.Contains(body, "Rust Link") {
		t.Fatalf("expected Rust Link in filtered results")
	}
	if strings.Contains(body, "Other Link") {
		t.Fatalf("expected Other Link not in filtered results")
	}
}

func TestTagsDisplayInItem(t *testing.T) {
	// Ensures tags are displayed in the item view.
	now := time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC)
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	_, _ = store.Create("https://example.com/tagged", "Tagged Link", now, []string{"go", "tutorial"})

	ui, err := New(store, func() time.Time { return now })
	if err != nil {
		t.Fatalf("new ui: %v", err)
	}

	server := httptest.NewServer(ui.Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("get active page: %v", err)
	}
	body, _ := readBody(resp)
	if !strings.Contains(body, `<div class="tags">go, tutorial</div>`) {
		t.Fatalf("expected tags to be displayed, got: %s", body)
	}
}
