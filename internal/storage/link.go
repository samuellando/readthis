package storage

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("link not found")

type Order int

const (
	OrderNewestFirst Order = iota
	OrderOldestFirst
)

type Link struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
	Archived  bool      `json:"archived"`
}
