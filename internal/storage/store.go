package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
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

// Create adds a new link with the provided URL, title, and timestamp.
func (s *FileStore) Create(url string, title string, createdAt time.Time) (Link, error) {
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
		}
		links = append(links, created)

		return s.writeLinks(links)
	})
	if err != nil {
		return Link{}, err
	}

	return created, nil
}

// List returns links filtered by archived status and sorted by order.
func (s *FileStore) List(order Order, archived bool) ([]Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var links []Link
	err := s.withFileLock(func() error {
		loaded, err := s.readLinks()
		if err != nil {
			return err
		}
		links = loaded
		return nil
	})
	if err != nil {
		return nil, err
	}

	filtered := make([]Link, 0, len(links))
	for _, link := range links {
		if link.Archived == archived {
			filtered = append(filtered, link)
		}
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

		found := false
		for i := range links {
			if links[i].ID == id {
				links[i].Archived = archived
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
