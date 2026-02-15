// Package storage provides file-backed persistence for links.
package storage

import (
	"errors"
	"time"
)

// ErrNotFound indicates a requested link does not exist.
var ErrNotFound = errors.New("link not found")

// Order controls list sorting.
type Order int

const (
	OrderNewestFirst Order = iota
	OrderOldestFirst
)

// Link represents a saved URL.
type Link struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
	Archived  bool      `json:"archived"`
}
