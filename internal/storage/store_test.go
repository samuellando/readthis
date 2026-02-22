package storage

import (
	"errors"
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

	first, err := store.Create("https://example.com/first", MissingTitle, firstTime, nil)
	if err != nil {
		t.Fatalf("create first link: %v", err)
	}
	second, err := store.Create("https://example.com/second", MissingTitle, secondTime, nil)
	if err != nil {
		t.Fatalf("create second link: %v", err)
	}

	links, err := store.List(OrderNewestFirst, false, nil)
	if err != nil {
		t.Fatalf("list newest first: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(links))
	}
	if links[0].ID != second.ID || links[1].ID != first.ID {
		t.Fatalf("unexpected order for newest first")
	}

	links, err = store.List(OrderOldestFirst, false, nil)
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

	created, err := store.Create("https://example.com/title", "Example Title", time.Now(), nil)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}
	if created.Title != "Example Title" {
		t.Fatalf("expected title to be preserved")
	}

	links, err := store.List(OrderNewestFirst, false, nil)
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

	created, err := store.Create("https://example.com/archive", MissingTitle, time.Now(), nil)
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

	archivedLinks, err := store.List(OrderNewestFirst, true, nil)
	if err != nil {
		t.Fatalf("list archived: %v", err)
	}
	if len(archivedLinks) != 0 {
		t.Fatalf("expected no archived links, got %d", len(archivedLinks))
	}

	activeLinks, err := store.List(OrderNewestFirst, false, nil)
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

	created, err := store.Create("https://example.com/persist", MissingTitle, time.Now(), nil)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	reloaded := NewFileStore(path)
	links, err := reloaded.List(OrderNewestFirst, false, nil)
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
			_, err := store.Create("https://example.com/concurrent", MissingTitle, base.Add(time.Duration(index)*time.Minute), nil)
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

	links, err := store.List(OrderNewestFirst, false, nil)
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

	created, err := store.Create("https://example.com/update", "Old Title", time.Now(), nil)
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

	links, err := store.List(OrderNewestFirst, false, nil)
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

	created, err := store.Create("https://example.com/get", "Get Title", time.Now(), nil)
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

	created, err := store.Create("https://example.com/delete", "Delete Title", time.Now(), nil)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	if err := store.Delete(created.ID); err != nil {
		t.Fatalf("delete link: %v", err)
	}

	links, err := store.List(OrderNewestFirst, false, nil)
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(links) != 0 {
		t.Fatalf("expected no links, got %d", len(links))
	}
}

func TestFileStoreArchiveClearsExpiredRevisitAt(t *testing.T) {
	// Ensures archiving a link clears expired revisit date.
	store := NewFileStore(filepath.Join(t.TempDir(), "links.json"))

	now := time.Now()
	pastTime := now.Add(-24 * time.Hour)

	created, err := store.Create("https://example.com/revisit", "Revisit Title", now, nil)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	_, err = store.SetRevisitAt(created.ID, pastTime)
	if err != nil {
		t.Fatalf("set revisit: %v", err)
	}

	archived, err := store.Archive(created.ID)
	if err != nil {
		t.Fatalf("archive link: %v", err)
	}
	if archived.RevisitAt != nil {
		t.Fatalf("expected revisit to be cleared after archiving")
	}
}

func TestFileStoreArchivePreservesFutureRevisitAt(t *testing.T) {
	// Ensures archiving a link preserves future revisit date.
	store := NewFileStore(filepath.Join(t.TempDir(), "links.json"))

	now := time.Now()
	futureTime := now.Add(24 * time.Hour)

	created, err := store.Create("https://example.com/revisit", "Revisit Title", now, nil)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	_, err = store.SetRevisitAt(created.ID, futureTime)
	if err != nil {
		t.Fatalf("set revisit: %v", err)
	}

	archived, err := store.Archive(created.ID)
	if err != nil {
		t.Fatalf("archive link: %v", err)
	}
	if archived.RevisitAt == nil {
		t.Fatalf("expected revisit to be preserved after archiving")
	}
	if !archived.RevisitAt.Equal(futureTime) {
		t.Fatalf("expected revisit time to be preserved")
	}
}

func TestFileStoreSetRevisitAt(t *testing.T) {
	// Ensures SetRevisitAt sets the revisit time and archives the link.
	store := NewFileStore(filepath.Join(t.TempDir(), "links.json"))

	now := time.Now()
	futureTime := now.Add(7 * 24 * time.Hour)

	created, err := store.Create("https://example.com/revisit", "Revisit Title", now, nil)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	updated, err := store.SetRevisitAt(created.ID, futureTime)
	if err != nil {
		t.Fatalf("set revisit: %v", err)
	}
	if updated.RevisitAt == nil {
		t.Fatalf("expected revisit_at to be set")
	}
	if !updated.RevisitAt.Equal(futureTime) {
		t.Fatalf("expected revisit_at to match")
	}
	if !updated.Archived {
		t.Fatalf("expected link to be archived")
	}
}

func TestFileStoreListAutoUnarchivesExpiredRevisit(t *testing.T) {
	// Ensures List auto-unarchives links when revisit time has passed.
	store := NewFileStore(filepath.Join(t.TempDir(), "links.json"))

	now := time.Now()
	pastTime := now.Add(-1 * time.Hour)

	created, err := store.Create("https://example.com/revisit", "Revisit Title", now, nil)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	_, err = store.SetRevisitAt(created.ID, pastTime)
	if err != nil {
		t.Fatalf("set revisit: %v", err)
	}

	archivedLinks, err := store.List(OrderNewestFirst, true, nil)
	if err != nil {
		t.Fatalf("list archived: %v", err)
	}
	if len(archivedLinks) != 0 {
		t.Fatalf("expected no archived links after auto-unarchive, got %d", len(archivedLinks))
	}

	activeLinks, err := store.List(OrderNewestFirst, false, nil)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(activeLinks) != 1 {
		t.Fatalf("expected 1 active link after auto-unarchive, got %d", len(activeLinks))
	}
	if activeLinks[0].RevisitAt != nil {
		t.Fatalf("expected revisit_at to be cleared after auto-unarchive")
	}
}

func TestFileStoreListPreservesFutureRevisit(t *testing.T) {
	// Ensures List does not unarchive links with future revisit time.
	store := NewFileStore(filepath.Join(t.TempDir(), "links.json"))

	now := time.Now()
	futureTime := now.Add(24 * time.Hour)

	created, err := store.Create("https://example.com/revisit", "Revisit Title", now, nil)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	_, err = store.SetRevisitAt(created.ID, futureTime)
	if err != nil {
		t.Fatalf("set revisit: %v", err)
	}

	archivedLinks, err := store.List(OrderNewestFirst, true, nil)
	if err != nil {
		t.Fatalf("list archived: %v", err)
	}
	if len(archivedLinks) != 1 {
		t.Fatalf("expected 1 archived link, got %d", len(archivedLinks))
	}
}

func TestFileStoreCreateWithTags(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "links.json"))

	tags := []string{"go", "tutorial"}
	created, err := store.Create("https://example.com/go", "Go Tutorial", time.Now(), tags)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}
	if len(created.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(created.Tags))
	}
	if created.Tags[0] != "go" || created.Tags[1] != "tutorial" {
		t.Fatalf("unexpected tags: %v", created.Tags)
	}

	links, err := store.List(OrderNewestFirst, false, nil)
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if len(links[0].Tags) != 2 {
		t.Fatalf("expected 2 tags on stored link, got %d", len(links[0].Tags))
	}
}

func TestFileStoreSetTags(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "links.json"))

	created, err := store.Create("https://example.com", "Example", time.Now(), nil)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	updated, err := store.SetTags(created.ID, []string{"tag1", "tag2"})
	if err != nil {
		t.Fatalf("set tags: %v", err)
	}
	if len(updated.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(updated.Tags))
	}

	link, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("get link: %v", err)
	}
	if len(link.Tags) != 2 {
		t.Fatalf("expected 2 tags on retrieved link, got %d", len(link.Tags))
	}
}

func TestFileStoreAddTags(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "links.json"))

	created, err := store.Create("https://example.com", "Example", time.Now(), []string{"existing"})
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	updated, err := store.AddTags(created.ID, []string{"new"})
	if err != nil {
		t.Fatalf("add tags: %v", err)
	}
	if len(updated.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(updated.Tags))
	}

	updated, err = store.AddTags(created.ID, []string{"existing", "another"})
	if err != nil {
		t.Fatalf("add tags again: %v", err)
	}
	if len(updated.Tags) != 3 {
		t.Fatalf("expected 3 tags after merge, got %d", len(updated.Tags))
	}
}

func TestFileStoreGetTags(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "links.json"))

	created, err := store.Create("https://example.com", "Example", time.Now(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	tags, err := store.GetTags(created.ID)
	if err != nil {
		t.Fatalf("get tags: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}

	_, err = store.GetTags("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for nonexistent link, got %v", err)
	}
}

func TestFileStoreListFilterByTags(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "links.json"))

	_, err := store.Create("https://example.com/go", "Go", time.Now(), []string{"go", "tutorial"})
	if err != nil {
		t.Fatalf("create link 1: %v", err)
	}
	_, err = store.Create("https://example.com/rust", "Rust", time.Now(), []string{"rust", "tutorial"})
	if err != nil {
		t.Fatalf("create link 2: %v", err)
	}
	_, err = store.Create("https://example.com/other", "Other", time.Now(), []string{"misc"})
	if err != nil {
		t.Fatalf("create link 3: %v", err)
	}
	_, err = store.Create("https://example.com/notag", "No Tag", time.Now(), nil)
	if err != nil {
		t.Fatalf("create link 4: %v", err)
	}

	links, err := store.List(OrderNewestFirst, false, nil)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(links) != 4 {
		t.Fatalf("expected 4 links with no filter, got %d", len(links))
	}

	links, err = store.List(OrderNewestFirst, false, []string{"tutorial"})
	if err != nil {
		t.Fatalf("list by tutorial tag: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("expected 2 links with tutorial tag, got %d", len(links))
	}

	links, err = store.List(OrderNewestFirst, false, []string{"go", "rust"})
	if err != nil {
		t.Fatalf("list by go or rust tag: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("expected 2 links with go or rust tag, got %d", len(links))
	}

	links, err = store.List(OrderNewestFirst, false, []string{"nonexistent"})
	if err != nil {
		t.Fatalf("list by nonexistent tag: %v", err)
	}
	if len(links) != 0 {
		t.Fatalf("expected 0 links with nonexistent tag, got %d", len(links))
	}
}
