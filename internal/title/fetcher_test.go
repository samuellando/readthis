package title

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchExtractsTitle(t *testing.T) {
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><head><title>Fetcher Page</title></head><body>Hello</body></html>"))
	}))
	defer page.Close()

	title, err := Fetch(context.Background(), page.URL)
	if err != nil {
		t.Fatalf("fetch title: %v", err)
	}
	if title != "Fetcher Page" {
		t.Fatalf("expected title, got %q", title)
	}
}
