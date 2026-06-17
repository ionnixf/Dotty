// Package storage persists the set of packages Dotty has installed.
// The format is a single JSON file (see the README/spec). There is no
// SQLite by design: the data set is tiny and a JSON file is trivially
// inspectable and editable by humans.
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

// Record describes one installed package. Target is stored as the user wrote
// it in the catalog (commonly with a leading "~") so the file remains readable.
type Record struct {
	Name        string    `json:"name"`
	Repo        string    `json:"repo"`
	Source      string    `json:"source"`
	Target      string    `json:"target"`
	InstalledAt time.Time `json:"installed_at"`
}

// Database is the on-disk structure.
type Database struct {
	Packages []Record `json:"packages"`
}

// Store reads and writes the installed-package database at one path. It is
// safe for concurrent use by a single goroutine; the TUI only ever calls it
// from Update, which is serialised by Bubble Tea.
type Store struct {
	path string
}

// New returns a Store backed by the given file path.
func New(path string) *Store {
	return &Store{path: path}
}

// Load reads the database. A missing file is treated as an empty database
// so Dotty works before any package is installed.
func (s *Store) Load() (Database, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return Database{Packages: []Record{}}, nil
		}
		return Database{}, fmt.Errorf("read installed database %s: %w", s.path, err)
	}

	var db Database
	if err := json.Unmarshal(raw, &db); err != nil {
		return Database{}, fmt.Errorf("parse installed database %s: %w", s.path, err)
	}
	if db.Packages == nil {
		db.Packages = []Record{}
	}
	return db, nil
}

// Save writes the database atomically via temp-file + rename.
func (s *Store) Save(db Database) error {
	if db.Packages == nil {
		db.Packages = []Record{}
	}
	raw, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return fmt.Errorf("encode installed database: %w", err)
	}
	raw = append(raw, '\n')

	dir := fileDir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create data dir %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".installed-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp database: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write database: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("commit database: %w", err)
	}
	return nil
}

// fileDir returns the directory portion of path, or "." if none.
func fileDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == os.PathSeparator {
			if i == 0 {
				return string(os.PathSeparator)
			}
			return path[:i]
		}
	}
	return "."
}

// Add inserts or replaces a record keyed by package name and persists.
func (s *Store) Add(rec Record) error {
	db, err := s.Load()
	if err != nil {
		return err
	}
	if rec.InstalledAt.IsZero() {
		rec.InstalledAt = time.Now().UTC()
	}
	replaced := false
	for i := range db.Packages {
		if db.Packages[i].Name == rec.Name {
			db.Packages[i] = rec
			replaced = true
			break
		}
	}
	if !replaced {
		db.Packages = append(db.Packages, rec)
	}
	return s.Save(db)
}

// Remove deletes the record with the given name and persists. It is not an
// error if the name is absent; the result is simply the unchanged database.
func (s *Store) Remove(name string) error {
	db, err := s.Load()
	if err != nil {
		return err
	}
	next := db.Packages[:0]
	for _, r := range db.Packages {
		if r.Name != name {
			next = append(next, r)
		}
	}
	db.Packages = next
	return s.Save(db)
}

// Find returns the record for name and whether it exists.
func (s *Store) Find(name string) (Record, bool, error) {
	db, err := s.Load()
	if err != nil {
		return Record{}, false, err
	}
	for _, r := range db.Packages {
		if r.Name == name {
			return r, true, nil
		}
	}
	return Record{}, false, nil
}

// All returns every record, sorted by name for stable display.
func (s *Store) All() ([]Record, error) {
	db, err := s.Load()
	if err != nil {
		return nil, err
	}
	sort.Slice(db.Packages, func(i, j int) bool {
		return db.Packages[i].Name < db.Packages[j].Name
	})
	return db.Packages, nil
}
