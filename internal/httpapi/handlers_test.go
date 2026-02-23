package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

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

func TestDeleteLink(t *testing.T) {
	// Ensures API can delete a link.
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><head><title>Delete Page</title></head><body>Hello</body></html>"))
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

	deleteReq, err := http.NewRequest(http.MethodDelete, server.URL+"/api/links/"+created.ID, nil)
	if err != nil {
		t.Fatalf("build delete request: %v", err)
	}
	resp, err = http.DefaultClient.Do(deleteReq)
	if err != nil {
		t.Fatalf("delete request failed: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		resp.Body.Close()
		t.Fatalf("expected status 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp, err = http.Get(server.URL + "/api/links?archived=false&order=newest")
	if err != nil {
		t.Fatalf("list request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	var links []storage.Link
	if err := json.NewDecoder(resp.Body).Decode(&links); err != nil {
		resp.Body.Close()
		t.Fatalf("decode list response: %v", err)
	}
	resp.Body.Close()
	if len(links) != 0 {
		t.Fatalf("expected no links, got %d", len(links))
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

func TestCreateWithRevisitAt(t *testing.T) {
	// Ensures API can create a link with revisit_at.
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	api := New(store)
	server := httptest.NewServer(api.Handler())
	defer server.Close()

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	revisitAt := futureTime.Format(time.RFC3339)

	createBody := []byte(`{"url":"https://example.com","title":"Test Link","revisit_at":"` + revisitAt + `"}`)
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

	if created.RevisitAt == nil {
		t.Fatalf("expected revisit_at to be set")
	}
	if !created.RevisitAt.Equal(futureTime) {
		t.Fatalf("expected revisit_at to match, got %v", created.RevisitAt)
	}
	if !created.Archived {
		t.Fatalf("expected link to be archived when revisit_at is set")
	}
}

func TestRevisitEndpoint(t *testing.T) {
	// Ensures API can set revisit time via revisit endpoint.
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	api := New(store)
	server := httptest.NewServer(api.Handler())
	defer server.Close()

	createBody := []byte(`{"url":"https://example.com","title":"Test Link"}`)
	resp, err := http.Post(server.URL+"/api/links", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	var created storage.Link
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		resp.Body.Close()
		t.Fatalf("decode create response: %v", err)
	}
	resp.Body.Close()

	futureTime := time.Now().Add(7 * 24 * time.Hour).UTC().Truncate(time.Second)
	revisitAt := futureTime.Format(time.RFC3339)

	revisitBody := []byte(`{"revisit_at":"` + revisitAt + `"}`)
	resp, err = http.Post(server.URL+"/api/links/"+created.ID+"/revisit", "application/json", bytes.NewReader(revisitBody))
	if err != nil {
		t.Fatalf("revisit request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	var updated storage.Link
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		resp.Body.Close()
		t.Fatalf("decode revisit response: %v", err)
	}
	resp.Body.Close()

	if updated.RevisitAt == nil {
		t.Fatalf("expected revisit_at to be set")
	}
	if !updated.RevisitAt.Equal(futureTime) {
		t.Fatalf("expected revisit_at to match, got %v", updated.RevisitAt)
	}
	if !updated.Archived {
		t.Fatalf("expected link to be archived")
	}
}

func TestRevisitEndpointRequiresRevisitAt(t *testing.T) {
	// Ensures revisit endpoint requires revisit_at.
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	api := New(store)
	server := httptest.NewServer(api.Handler())
	defer server.Close()

	createBody := []byte(`{"url":"https://example.com","title":"Test Link"}`)
	resp, err := http.Post(server.URL+"/api/links", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	var created storage.Link
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		resp.Body.Close()
		t.Fatalf("decode create response: %v", err)
	}
	resp.Body.Close()

	revisitBody := []byte(`{}`)
	resp, err = http.Post(server.URL+"/api/links/"+created.ID+"/revisit", "application/json", bytes.NewReader(revisitBody))
	if err != nil {
		t.Fatalf("revisit request failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		resp.Body.Close()
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestCreateWithTags(t *testing.T) {
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	api := New(store)
	server := httptest.NewServer(api.Handler())
	defer server.Close()

	createBody := []byte(`{"url":"https://example.com","title":"Test Link","tags":["go","tutorial"]}`)
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

	if len(created.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(created.Tags))
	}
	if created.Tags[0] != "go" || created.Tags[1] != "tutorial" {
		t.Fatalf("unexpected tags: %v", created.Tags)
	}
}

func TestCreateWithoutTags(t *testing.T) {
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	api := New(store)
	server := httptest.NewServer(api.Handler())
	defer server.Close()

	createBody := []byte(`{"url":"https://example.com","title":"Test Link"}`)
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

	if len(created.Tags) != 0 {
		t.Fatalf("expected 0 tags, got %d", len(created.Tags))
	}
}

func TestCreateNormalizesTags(t *testing.T) {
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	api := New(store)
	server := httptest.NewServer(api.Handler())
	defer server.Close()

	createBody := []byte(`{"url":"https://example.com","title":"Test Link","tags":["go","","go","  rust  ",""]}`)
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

	if len(created.Tags) != 2 {
		t.Fatalf("expected 2 tags after normalization, got %d: %v", len(created.Tags), created.Tags)
	}
	if created.Tags[0] != "go" || created.Tags[1] != "rust" {
		t.Fatalf("unexpected tags after normalization: %v", created.Tags)
	}
}

func TestGetTags(t *testing.T) {
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	api := New(store)
	server := httptest.NewServer(api.Handler())
	defer server.Close()

	createBody := []byte(`{"url":"https://example.com","title":"Test Link","tags":["tag1","tag2"]}`)
	resp, err := http.Post(server.URL+"/api/links", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	var created storage.Link
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		resp.Body.Close()
		t.Fatalf("decode create response: %v", err)
	}
	resp.Body.Close()

	resp, err = http.Get(server.URL + "/api/links/" + created.ID + "/tags")
	if err != nil {
		t.Fatalf("get tags request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	var tags []string
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		resp.Body.Close()
		t.Fatalf("decode tags response: %v", err)
	}
	resp.Body.Close()

	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
}

func TestGetTagsNotFound(t *testing.T) {
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	api := New(store)
	server := httptest.NewServer(api.Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/links/nonexistent/tags")
	if err != nil {
		t.Fatalf("get tags request failed: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		resp.Body.Close()
		t.Fatalf("expected status 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestSetTags(t *testing.T) {
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	api := New(store)
	server := httptest.NewServer(api.Handler())
	defer server.Close()

	createBody := []byte(`{"url":"https://example.com","title":"Test Link"}`)
	resp, err := http.Post(server.URL+"/api/links", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	var created storage.Link
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		resp.Body.Close()
		t.Fatalf("decode create response: %v", err)
	}
	resp.Body.Close()

	setTagsBody := []byte(`{"tags":["new1","new2"]}`)
	resp, err = http.Post(server.URL+"/api/links/"+created.ID+"/tags", "application/json", bytes.NewReader(setTagsBody))
	if err != nil {
		t.Fatalf("set tags request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	var updated storage.Link
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		resp.Body.Close()
		t.Fatalf("decode set tags response: %v", err)
	}
	resp.Body.Close()

	if len(updated.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(updated.Tags))
	}
	if updated.Tags[0] != "new1" || updated.Tags[1] != "new2" {
		t.Fatalf("unexpected tags: %v", updated.Tags)
	}
}

func TestSetTagsNotFound(t *testing.T) {
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	api := New(store)
	server := httptest.NewServer(api.Handler())
	defer server.Close()

	setTagsBody := []byte(`{"tags":["tag"]}`)
	resp, err := http.Post(server.URL+"/api/links/nonexistent/tags", "application/json", bytes.NewReader(setTagsBody))
	if err != nil {
		t.Fatalf("set tags request failed: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		resp.Body.Close()
		t.Fatalf("expected status 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestSetTagsNormalizesTags(t *testing.T) {
	store := storage.NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	api := New(store)
	server := httptest.NewServer(api.Handler())
	defer server.Close()

	createBody := []byte(`{"url":"https://example.com","title":"Test Link"}`)
	resp, err := http.Post(server.URL+"/api/links", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	var created storage.Link
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		resp.Body.Close()
		t.Fatalf("decode create response: %v", err)
	}
	resp.Body.Close()

	setTagsBody := []byte(`{"tags":["go","","go","  rust  ",""]}`)
	resp, err = http.Post(server.URL+"/api/links/"+created.ID+"/tags", "application/json", bytes.NewReader(setTagsBody))
	if err != nil {
		t.Fatalf("set tags request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	var updated storage.Link
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		resp.Body.Close()
		t.Fatalf("decode set tags response: %v", err)
	}
	resp.Body.Close()

	if len(updated.Tags) != 2 {
		t.Fatalf("expected 2 tags after normalization, got %d: %v", len(updated.Tags), updated.Tags)
	}
	if updated.Tags[0] != "go" || updated.Tags[1] != "rust" {
		t.Fatalf("unexpected tags after normalization: %v", updated.Tags)
	}
}
