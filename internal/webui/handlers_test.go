package webui

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"readthis/internal/storage"
)

func TestActivePageGolden(t *testing.T) {
	// Ensures active list page renders consistently.
	now := time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC)
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	_, _ = store.Create("https://example.com/one", storage.MissingTitle, now.Add(-24*time.Hour))
	_, _ = store.Create("https://example.com/two", storage.MissingTitle, now.Add(-2*time.Hour))

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
	defer resp.Body.Close()

	assertGolden(t, resp, "active.html")
}

func TestArchivePageGolden(t *testing.T) {
	// Ensures archive page renders consistently.
	now := time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC)
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	created, _ := store.Create("https://example.com/old", storage.MissingTitle, now.Add(-48*time.Hour))
	_, _ = store.Archive(created.ID)

	ui, err := New(store, func() time.Time { return now })
	if err != nil {
		t.Fatalf("new ui: %v", err)
	}

	server := httptest.NewServer(ui.Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/archive")
	if err != nil {
		t.Fatalf("get archive page: %v", err)
	}
	defer resp.Body.Close()

	assertGolden(t, resp, "archive.html")
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

	links, err := store.List(storage.OrderNewestFirst, false)
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
	created, err := store.Create("https://example.com/archive", "Archive Title", now)
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

	archived, err := store.List(storage.OrderNewestFirst, true)
	if err != nil {
		t.Fatalf("list archived: %v", err)
	}
	if len(archived) != 1 || archived[0].ID != created.ID {
		t.Fatalf("expected archived link")
	}
}
