package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"readthis/internal/storage"
)

func TestCreateAndListLinks(t *testing.T) {
	// Ensures API can create a link and list active links.
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><head><title>Example Page</title></head><body>Hello</body></html>"))
	}))
	defer page.Close()

	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	api := New(store)
	server := httptest.NewServer(api.Handler())
	defer server.Close()

	createBody := []byte(`{"url":"` + page.URL + `"}`)
	resp, err := http.Post(server.URL+"/api/links", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp, err = http.Get(server.URL + "/api/links?archived=false&order=newest")
	if err != nil {
		t.Fatalf("list request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	var links []storage.Link
	if err := json.NewDecoder(resp.Body).Decode(&links); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
}

func TestCreateExtractsTitle(t *testing.T) {
	// Ensures create extracts title from page content.
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><head><title>Example Page</title></head><body>Hello</body></html>"))
	}))
	defer page.Close()

	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	api := New(store)
	server := httptest.NewServer(api.Handler())
	defer server.Close()

	createBody := []byte(`{"url":"` + page.URL + `"}`)
	resp, err := http.Post(server.URL+"/api/links", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		resp.Body.Close()
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}
	var created storage.Link
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		resp.Body.Close()
		t.Fatalf("decode create response: %v", err)
	}
	resp.Body.Close()

	if created.Title != "Example Page" {
		t.Fatalf("expected extracted title, got %q", created.Title)
	}
}

func TestCreateUsesProvidedTitle(t *testing.T) {
	// Ensures create uses provided title without fetching.
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	api := New(store)
	server := httptest.NewServer(api.Handler())
	defer server.Close()

	createBody := []byte(`{"url":"https://example.com","title":"Manual Title"}`)
	resp, err := http.Post(server.URL+"/api/links", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		resp.Body.Close()
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}
	var created storage.Link
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		resp.Body.Close()
		t.Fatalf("decode create response: %v", err)
	}
	resp.Body.Close()

	if created.Title != "Manual Title" {
		t.Fatalf("expected provided title, got %q", created.Title)
	}
}

func TestArchiveAndUnarchiveLink(t *testing.T) {
	// Ensures API can archive and unarchive links, and list archived only.
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><head><title>Archive Page</title></head><body>Hello</body></html>"))
	}))
	defer page.Close()

	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	api := New(store)
	server := httptest.NewServer(api.Handler())
	defer server.Close()

	createBody := []byte(`{"url":"` + page.URL + `"}`)
	resp, err := http.Post(server.URL+"/api/links", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}
	var created storage.Link
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		resp.Body.Close()
		t.Fatalf("decode create response: %v", err)
	}
	resp.Body.Close()

	resp, err = http.Post(server.URL+"/api/links/"+created.ID+"/archive", "application/json", nil)
	if err != nil {
		t.Fatalf("archive request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp, err = http.Get(server.URL + "/api/links?archived=true")
	if err != nil {
		t.Fatalf("list archived request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	var archived []storage.Link
	if err := json.NewDecoder(resp.Body).Decode(&archived); err != nil {
		resp.Body.Close()
		t.Fatalf("decode archived response: %v", err)
	}
	resp.Body.Close()
	if len(archived) != 1 || archived[0].ID != created.ID {
		t.Fatalf("expected archived link in list")
	}

	resp, err = http.Post(server.URL+"/api/links/"+created.ID+"/unarchive", "application/json", nil)
	if err != nil {
		t.Fatalf("unarchive request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp, err = http.Get(server.URL + "/api/links?archived=true")
	if err != nil {
		t.Fatalf("list archived request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	archived = nil
	if err := json.NewDecoder(resp.Body).Decode(&archived); err != nil {
		resp.Body.Close()
		t.Fatalf("decode archived response: %v", err)
	}
	resp.Body.Close()
	if len(archived) != 0 {
		t.Fatalf("expected no archived links, got %d", len(archived))
	}
}

func TestListValidation(t *testing.T) {
	// Ensures invalid query parameters are rejected.
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	api := New(store)
	server := httptest.NewServer(api.Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/links?archived=maybe")
	if err != nil {
		t.Fatalf("list request failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		resp.Body.Close()
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp, err = http.Get(server.URL + "/api/links?order=middle")
	if err != nil {
		t.Fatalf("list request failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		resp.Body.Close()
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}
