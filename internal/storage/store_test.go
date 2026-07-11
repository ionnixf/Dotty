package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreAddFindRemoveAll(t *testing.T) {
	tempDir := t.TempDir()
	dbFile := filepath.Join(tempDir, "installed.json")

	store := New(dbFile)

	// Initially empty
	records, err := store.All()
	if err != nil {
		t.Fatalf("All failed: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("Expected 0 records, got %d", len(records))
	}

	// Find non-existing
	_, ok, err := store.Find("pkg-a")
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if ok {
		t.Error("Expected pkg-a to be not found")
	}

	// Add record
	rec := Record{
		Name:        "pkg-a",
		Repo:        "https://example.com/pkg-a",
		Source:      "src",
		Target:      "~/.config/pkg-a",
		InstalledAt: time.Now().UTC(),
	}
	if err := store.Add(rec); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Find added record
	found, ok, err := store.Find("pkg-a")
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if !ok {
		t.Fatal("Expected to find pkg-a")
	}
	if found.Name != rec.Name || found.Repo != rec.Repo || found.Target != rec.Target {
		t.Errorf("Record mismatch: got %+v, want %+v", found, rec)
	}

	// Add another
	rec2 := Record{
		Name:   "pkg-b",
		Repo:   "https://example.com/pkg-b",
		Target: "~/.config/pkg-b",
	}
	if err := store.Add(rec2); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Check All sorted by name
	records, err = store.All()
	if err != nil {
		t.Fatalf("All failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("Expected 2 records, got %d", len(records))
	}
	if records[0].Name != "pkg-a" || records[1].Name != "pkg-b" {
		t.Errorf("Incorrect sorting: %+v", records)
	}

	// Remove pkg-a
	if err := store.Remove("pkg-a"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify removal
	_, ok, err = store.Find("pkg-a")
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if ok {
		t.Error("Expected pkg-a to be removed")
	}

	// Verify database persists to file
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		t.Error("Database file was not created on disk")
	}
}
