// Package httpapi exposes REST handlers for ReadThis.
package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"readthis/internal/storage"
	"readthis/internal/title"
)

// API provides HTTP handlers backed by a storage layer.
type API struct {
	store *storage.FileStore
}

// New constructs an API handler with the provided store.
func New(store *storage.FileStore) *API {
	return &API{store: store}
}

// Handler returns the HTTP mux for API routes.
func (api *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/links", api.handleLinks)
	mux.HandleFunc("/api/links/", api.handleLinkAction)
	return mux
}

// handleLinks routes list and create operations.
func (api *API) handleLinks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		api.handleList(w, r)
	case http.MethodPost:
		api.handleCreate(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleCreate creates a new link.
func (api *API) handleCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if payload.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}

	linkTitle := strings.TrimSpace(payload.Title)
	if linkTitle == "" {
		linkTitle = storage.MissingTitle
		if extracted, err := title.Fetch(r.Context(), payload.URL); err == nil && strings.TrimSpace(extracted) != "" {
			linkTitle = extracted
		} else if err != nil {
			log.Println(err)
		}
	}

	created, err := api.store.Create(payload.URL, linkTitle, time.Now().UTC())
	if err != nil {
		log.Println(err)
		writeError(w, http.StatusInternalServerError, "failed to create link")
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

// handleList returns links filtered by archived status.
func (api *API) handleList(w http.ResponseWriter, r *http.Request) {
	order := storage.OrderNewestFirst
	if value := r.URL.Query().Get("order"); value != "" {
		switch value {
		case "newest":
			order = storage.OrderNewestFirst
		case "oldest":
			order = storage.OrderOldestFirst
		default:
			writeError(w, http.StatusBadRequest, "order must be newest or oldest")
			return
		}
	}

	archived := false
	if value := r.URL.Query().Get("archived"); value != "" {
		parsed, err := parseBool(value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "archived must be true or false")
			return
		}
		archived = parsed
	}

	links, err := api.store.List(order, archived)
	if err != nil {
		log.Println(err)
		writeError(w, http.StatusInternalServerError, "failed to list links")
		return
	}

	writeJSON(w, http.StatusOK, links)
}

// handleLinkAction archives or unarchives a link.
func (api *API) handleLinkAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/links/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	linkID := parts[0]
	action := parts[1]

	var updated storage.Link
	var err error
	if action == "archive" {
		updated, err = api.store.Archive(linkID)
	} else if action == "unarchive" {
		updated, err = api.store.Unarchive(linkID)
	} else {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "link not found")
			return
		}
		log.Println(err)
		writeError(w, http.StatusInternalServerError, "failed to update link")
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

// parseBool parses a true/false string with validation.
func parseBool(value string) (bool, error) {
	switch strings.ToLower(value) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, errors.New("invalid bool")
	}
}

// writeJSON writes a JSON response with status.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
