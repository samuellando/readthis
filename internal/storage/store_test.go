package storage

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestFileStoreCreateAndListOrder(t *testing.T) {
	// Verifies create and ordering behavior for newest/oldest sorting.
	store := NewFileStore(filepath.Join(t.TempDir(), "links.json"))

	firstTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	secondTime := firstTime.Add(2 * time.Hour)

	first, err := store.Create("https://example.com/first", MissingTitle, firstTime)
	if err != nil {
		t.Fatalf("create first link: %v", err)
	}
	second, err := store.Create("https://example.com/second", MissingTitle, secondTime)
	if err != nil {
		t.Fatalf("create second link: %v", err)
	}

	links, err := store.List(OrderNewestFirst, false)
	if err != nil {
		t.Fatalf("list newest first: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(links))
	}
	if links[0].ID != second.ID || links[1].ID != first.ID {
		t.Fatalf("unexpected order for newest first")
	}

	links, err = store.List(OrderOldestFirst, false)
	if err != nil {
		t.Fatalf("list oldest first: %v", err)
	}
	if links[0].ID != first.ID || links[1].ID != second.ID {
		t.Fatalf("unexpected order for oldest first")
	}
	if links[0].Title != MissingTitle || links[1].Title != MissingTitle {
		t.Fatalf("expected missing title defaults")
	}
}

func TestFileStoreCreatePreservesTitle(t *testing.T) {
	// Ensures non-empty titles are preserved.
	store := NewFileStore(filepath.Join(t.TempDir(), "links.json"))

	created, err := store.Create("https://example.com/title", "Example Title", time.Now())
	if err != nil {
		t.Fatalf("create link: %v", err)
	}
	if created.Title != "Example Title" {
		t.Fatalf("expected title to be preserved")
	}

	links, err := store.List(OrderNewestFirst, false)
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].Title != "Example Title" {
		t.Fatalf("expected stored title to be preserved")
	}
}

func TestFileStoreArchiveAndUnarchive(t *testing.T) {
	// Verifies archive/unarchive updates and archived filtering.
	store := NewFileStore(filepath.Join(t.TempDir(), "links.json"))

	created, err := store.Create("https://example.com/archive", MissingTitle, time.Now())
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	archived, err := store.Archive(created.ID)
	if err != nil {
		t.Fatalf("archive link: %v", err)
	}
	if !archived.Archived {
		t.Fatalf("expected archived link")
	}

	unarchived, err := store.Unarchive(created.ID)
	if err != nil {
		t.Fatalf("unarchive link: %v", err)
	}
	if unarchived.Archived {
		t.Fatalf("expected unarchived link")
	}

	archivedLinks, err := store.List(OrderNewestFirst, true)
	if err != nil {
		t.Fatalf("list archived: %v", err)
	}
	if len(archivedLinks) != 0 {
		t.Fatalf("expected no archived links, got %d", len(archivedLinks))
	}

	activeLinks, err := store.List(OrderNewestFirst, false)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(activeLinks) != 1 || activeLinks[0].ID != created.ID {
		t.Fatalf("expected active link")
	}

	_, err = store.Archive("missing")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFileStorePersistsToDisk(t *testing.T) {
	// Ensures data is saved and reloadable from disk.
	path := filepath.Join(t.TempDir(), "links.json")
	store := NewFileStore(path)

	created, err := store.Create("https://example.com/persist", MissingTitle, time.Now())
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	reloaded := NewFileStore(path)
	links, err := reloaded.List(OrderNewestFirst, false)
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(links) != 1 || links[0].ID != created.ID {
		t.Fatalf("expected persisted link")
	}
}

func TestFileStoreConcurrentCreates(t *testing.T) {
	// Ensures concurrent creates do not corrupt data.
	store := NewFileStore(filepath.Join(t.TempDir(), "links.json"))
	base := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		index := i
		go func() {
			defer wg.Done()
			_, err := store.Create("https://example.com/concurrent", MissingTitle, base.Add(time.Duration(index)*time.Minute))
			if err != nil {
				t.Errorf("create link: %v", err)
			}
		}()
	}
	wdone := make(chan struct{})
	go func() {
		wg.Wait()
		close(wdone)
	}()
	select {
	case <-wdone:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for concurrent creates")
	}

	links, err := store.List(OrderNewestFirst, false)
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(links) != 20 {
		t.Fatalf("expected 20 links, got %d", len(links))
	}
}

func TestFileStoreUpdateTitle(t *testing.T) {
	// Ensures title updates persist.
	store := NewFileStore(filepath.Join(t.TempDir(), "links.json"))

	created, err := store.Create("https://example.com/update", "Old Title", time.Now())
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	updated, err := store.UpdateTitle(created.ID, "New Title")
	if err != nil {
		t.Fatalf("update title: %v", err)
	}
	if updated.Title != "New Title" {
		t.Fatalf("expected updated title")
	}

	links, err := store.List(OrderNewestFirst, false)
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].Title != "New Title" {
		t.Fatalf("expected stored title to update")
	}
}

func TestFileStoreGet(t *testing.T) {
	// Ensures Get returns the stored link by ID.
	store := NewFileStore(filepath.Join(t.TempDir(), "links.json"))

	created, err := store.Create("https://example.com/get", "Get Title", time.Now())
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	fetched, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("get link: %v", err)
	}
	if fetched.ID != created.ID {
		t.Fatalf("expected matching id")
	}
	if fetched.Title != "Get Title" {
		t.Fatalf("expected matching title")
	}
}

func TestFileStoreDelete(t *testing.T) {
	// Ensures Delete removes the link.
	store := NewFileStore(filepath.Join(t.TempDir(), "links.json"))

	created, err := store.Create("https://example.com/delete", "Delete Title", time.Now())
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	if err := store.Delete(created.ID); err != nil {
		t.Fatalf("delete link: %v", err)
	}

	links, err := store.List(OrderNewestFirst, false)
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(links) != 0 {
		t.Fatalf("expected no links, got %d", len(links))
	}
}
