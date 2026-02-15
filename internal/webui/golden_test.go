package webui

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// assertGolden compares the response body to a golden file after normalization.
func assertGolden(t *testing.T, resp *http.Response, filename string) {
	t.Helper()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	normalized := normalizeUUIDs(string(data))

	path := filepath.Join("testdata", filename)
	golden, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file: %v", err)
	}

	if normalized != string(golden) {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", filename, normalized, string(golden))
	}
}

// normalizeUUIDs replaces UUIDs to stabilize golden comparisons.
func normalizeUUIDs(input string) string {
	pattern := regexp.MustCompile(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`)
	return pattern.ReplaceAllString(input, "UUID")
}
