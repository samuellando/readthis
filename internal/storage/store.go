package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

// FileStore persists links to a JSON file with locking for safety.
type FileStore struct {
	path     string
	lockPath string
	mu       sync.Mutex
}

// NewFileStore initializes a file-backed store at the provided path.
func NewFileStore(path string) *FileStore {
	return &FileStore{
		path:     path,
		lockPath: path + ".lock",
	}
}

// Create adds a new link with the provided URL, title, timestamp, and tags.
func (s *FileStore) Create(url string, title string, createdAt time.Time, tags []string) (Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var created Link
	err := s.withFileLock(func() error {
		links, err := s.readLinks()
		if err != nil {
			return err
		}

		created = Link{
			ID:        uuid.NewString(),
			URL:       url,
			Title:     title,
			CreatedAt: createdAt,
			Archived:  false,
			Tags:      tags,
		}
		links = append(links, created)

		return s.writeLinks(links)
	})
	if err != nil {
		return Link{}, err
	}

	return created, nil
}

// List returns links filtered by archived status and tags, sorted by order.
func (s *FileStore) List(order Order, archived bool, tags []string) ([]Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var links []Link
	err := s.withFileLock(func() error {
		loaded, err := s.readLinks()
		if err != nil {
			return err
		}
		links = loaded

		now := time.Now()
		modified := false
		for i := range links {
			if links[i].Archived && links[i].RevisitAt != nil && !links[i].RevisitAt.After(now) {
				links[i].Archived = false
				links[i].RevisitAt = nil
				modified = true
			}
		}
		if modified {
			if err := s.writeLinks(links); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	filtered := make([]Link, 0, len(links))
	for _, link := range links {
		if link.Archived != archived {
			continue
		}
		if len(tags) > 0 {
			hasTag := false
			for _, t := range tags {
				if slices.Contains(link.Tags, t) {
					hasTag = true
					break
				}
			}
			if !hasTag {
				continue
			}
		}
		filtered = append(filtered, link)
	}
	links = filtered

	sort.Slice(links, func(i, j int) bool {
		if order == OrderOldestFirst {
			return links[i].CreatedAt.Before(links[j].CreatedAt)
		}
		return links[i].CreatedAt.After(links[j].CreatedAt)
	})

	return links, nil
}

// Archive marks a link as archived.
func (s *FileStore) Archive(id string) (Link, error) {
	return s.setArchived(id, true)
}

// Unarchive marks a link as active.
func (s *FileStore) Unarchive(id string) (Link, error) {
	return s.setArchived(id, false)
}

// Delete removes a link by ID.
func (s *FileStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.withFileLock(func() error {
		links, err := s.readLinks()
		if err != nil {
			return err
		}

		filtered := links[:0]
		found := false
		for _, link := range links {
			if link.ID == id {
				found = true
				continue
			}
			filtered = append(filtered, link)
		}
		if !found {
			return ErrNotFound
		}

		return s.writeLinks(filtered)
	})
}

// Get returns a link by ID.
func (s *FileStore) Get(id string) (Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var found Link
	err := s.withFileLock(func() error {
		links, err := s.readLinks()
		if err != nil {
			return err
		}

		for _, link := range links {
			if link.ID == id {
				found = link
				return nil
			}
		}

		return ErrNotFound
	})
	if err != nil {
		return Link{}, err
	}

	return found, nil
}

// UpdateTitle updates the title for a link by ID.
func (s *FileStore) UpdateTitle(id string, title string) (Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cleanTitle := strings.TrimSpace(title)
	if cleanTitle == "" {
		cleanTitle = MissingTitle
	}

	var updated Link
	err := s.withFileLock(func() error {
		links, err := s.readLinks()
		if err != nil {
			return err
		}

		found := false
		for i := range links {
			if links[i].ID == id {
				links[i].Title = cleanTitle
				updated = links[i]
				found = true
				break
			}
		}
		if !found {
			return ErrNotFound
		}

		return s.writeLinks(links)
	})
	if err != nil {
		return Link{}, err
	}

	return updated, nil
}

// setArchived updates the archived flag for a link by ID.
func (s *FileStore) setArchived(id string, archived bool) (Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var updated Link
	err := s.withFileLock(func() error {
		links, err := s.readLinks()
		if err != nil {
			return err
		}

		now := time.Now()
		found := false
		for i := range links {
			if links[i].ID == id {
				links[i].Archived = archived
				if archived && links[i].RevisitAt != nil && !links[i].RevisitAt.After(now) {
					links[i].RevisitAt = nil
				}
				updated = links[i]
				found = true
				break
			}
		}
		if !found {
			return ErrNotFound
		}

		return s.writeLinks(links)
	})
	if err != nil {
		return Link{}, err
	}

	return updated, nil
}

// SetRevisitAt sets the revisit time for a link and archives it.
func (s *FileStore) SetRevisitAt(id string, revisitAt time.Time) (Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var updated Link
	err := s.withFileLock(func() error {
		links, err := s.readLinks()
		if err != nil {
			return err
		}

		found := false
		for i := range links {
			if links[i].ID == id {
				links[i].RevisitAt = &revisitAt
				links[i].Archived = true
				updated = links[i]
				found = true
				break
			}
		}
		if !found {
			return ErrNotFound
		}

		return s.writeLinks(links)
	})
	if err != nil {
		return Link{}, err
	}

	return updated, nil
}

// SetTags replaces all tags for a link.
func (s *FileStore) SetTags(id string, tags []string) (Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var updated Link
	err := s.withFileLock(func() error {
		links, err := s.readLinks()
		if err != nil {
			return err
		}

		found := false
		for i := range links {
			if links[i].ID == id {
				links[i].Tags = tags
				updated = links[i]
				found = true
				break
			}
		}
		if !found {
			return ErrNotFound
		}

		return s.writeLinks(links)
	})
	if err != nil {
		return Link{}, err
	}

	return updated, nil
}

// AddTags adds new tags to a link, merging with existing tags.
func (s *FileStore) AddTags(id string, tagsToAdd []string) (Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var updated Link
	err := s.withFileLock(func() error {
		links, err := s.readLinks()
		if err != nil {
			return err
		}

		found := false
		for i := range links {
			if links[i].ID == id {
				existing := make(map[string]bool)
				for _, t := range links[i].Tags {
					existing[t] = true
				}
				for _, t := range tagsToAdd {
					if !existing[t] {
						links[i].Tags = append(links[i].Tags, t)
					}
				}
				updated = links[i]
				found = true
				break
			}
		}
		if !found {
			return ErrNotFound
		}

		return s.writeLinks(links)
	})
	if err != nil {
		return Link{}, err
	}

	return updated, nil
}

// GetTags returns all tags for a link.
func (s *FileStore) GetTags(id string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var tags []string
	err := s.withFileLock(func() error {
		links, err := s.readLinks()
		if err != nil {
			return err
		}

		found := false
		for _, link := range links {
			if link.ID == id {
				tags = link.Tags
				found = true
				break
			}
		}
		if !found {
			return ErrNotFound
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return tags, nil
}

// withFileLock executes a function while holding an OS file lock.
func (s *FileStore) withFileLock(fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}

	lockFile, err := os.OpenFile(s.lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer func() {
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
	}()

	return fn()
}

// readLinks loads links from disk, returning empty when missing.
func (s *FileStore) readLinks() ([]Link, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Link{}, nil
		}
		return nil, err
	}

	if len(data) == 0 {
		return []Link{}, nil
	}

	var links []Link
	if err := json.Unmarshal(data, &links); err != nil {
		return nil, err
	}

	for i := range links {
		if links[i].Title == "" {
			links[i].Title = MissingTitle
		}
	}

	return links, nil
}

// writeLinks atomically writes links to disk.
func (s *FileStore) writeLinks(links []Link) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(links, "", "  ")
	if err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(s.path), "links-*.json")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	return os.Rename(tmpFile.Name(), s.path)
}
