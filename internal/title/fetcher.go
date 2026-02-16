// Package title fetches article titles from URLs.
package title

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	readability "codeberg.org/readeck/go-readability/v2"
)

const (
	maxHTMLBytes = 5 << 20
	fetchTimeout = 10 * time.Second
)

// Fetch downloads the page and extracts a title.
func Fetch(ctx context.Context, target string) (string, error) {
	if strings.TrimSpace(target) == "" {
		return "", errors.New("url is required")
	}

	parsed, err := url.Parse(target)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "readthis/1.0")

	client := &http.Client{Timeout: fetchTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	limited := io.LimitReader(resp.Body, maxHTMLBytes)
	article, err := readability.FromReader(limited, parsed)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(article.Title()), nil
}
