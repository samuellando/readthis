package webui

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"readthis/internal/storage"
)

func TestActivePageGolden(t *testing.T) {
	// Ensures active list page renders consistently.
	now := time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC)
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	_, _ = store.Create("https://example.com/one", now.Add(-24*time.Hour))
	_, _ = store.Create("https://example.com/two", now.Add(-2*time.Hour))

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
	created, _ := store.Create("https://example.com/old", now.Add(-48*time.Hour))
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
