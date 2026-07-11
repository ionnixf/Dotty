package remover

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ion/dotty/internal/config"
	"github.com/ion/dotty/internal/storage"
)

func TestRemover(t *testing.T) {
	tempDir := t.TempDir()
	paths := config.Paths{
		ConfigDir: filepath.Join(tempDir, "config"),
		DataDir:   filepath.Join(tempDir, "data"),
		RepoDir:   filepath.Join(tempDir, "data", "repos"),
	}
	home := filepath.Join(tempDir, "home")
	_ = os.MkdirAll(home, 0o755)

	store := storage.New(filepath.Join(paths.DataDir, "installed.json"))
	rem := New(store, paths, home)

	// 1. Remove non-existent package
	err := rem.Remove("pkg-missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}

	// Setup a package and target symlink
	pkgName := "my-app"
	targetRel := "~/my-app-target"
	targetAbs := filepath.Join(home, "my-app-target")

	// Create dummy source file and symlink
	sourceFile := filepath.Join(tempDir, "source-file")
	if err := os.WriteFile(sourceFile, []byte("hello"), 0o644); err != nil {
		t.Fatalf("create source file: %v", err)
	}
	if err := os.Symlink(sourceFile, targetAbs); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	// Add to storage
	rec := storage.Record{
		Name:   pkgName,
		Repo:   "https://example.com/repo",
		Target: targetRel,
	}
	if err := store.Add(rec); err != nil {
		t.Fatalf("db save: %v", err)
	}

	// 2. Remove happy path
	if err := rem.Remove(pkgName); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify symlink is gone
	if _, err := os.Lstat(targetAbs); !os.IsNotExist(err) {
		t.Error("expected symlink to be deleted")
	}

	// Verify record is gone
	_, ok, err := store.Find(pkgName)
	if err != nil || ok {
		t.Errorf("expected DB record to be deleted: ok=%v, err=%v", ok, err)
	}

	// 3. Target is not a symlink
	// Re-add to DB
	if err := store.Add(rec); err != nil {
		t.Fatalf("db save: %v", err)
	}
	// Create a real file at target instead of a symlink
	if err := os.WriteFile(targetAbs, []byte("not a symlink"), 0o644); err != nil {
		t.Fatalf("create real file: %v", err)
	}

	err = rem.Remove(pkgName)
	if !errors.Is(err, ErrNotASymlink) {
		t.Errorf("expected ErrNotASymlink, got: %v", err)
	}

	// Clean up real file
	_ = os.Remove(targetAbs)
}
