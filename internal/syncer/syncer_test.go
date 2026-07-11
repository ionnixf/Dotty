package syncer

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ion/dotty/internal/config"
	"github.com/ion/dotty/internal/git"
	"github.com/ion/dotty/internal/storage"
)

// setupLocalGitRepo helper to create a dummy git repository
func setupLocalGitRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}

	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %v failed: %v", args, err)
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test User")
	
	// Create a dummy file and commit it
	dummyFile := filepath.Join(dir, "my-config-file")
	if err := os.WriteFile(dummyFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("write dummy file: %v", err)
	}

	runGit("add", "my-config-file")
	runGit("commit", "-m", "initial commit")
}

func TestSyncerCheck(t *testing.T) {
	tempDir := t.TempDir()
	paths := config.Paths{
		ConfigDir: filepath.Join(tempDir, "config"),
		DataDir:   filepath.Join(tempDir, "data"),
		RepoDir:   filepath.Join(tempDir, "data", "repos"),
	}
	home := filepath.Join(tempDir, "home")
	_ = os.MkdirAll(home, 0o755)

	store := storage.New(filepath.Join(paths.DataDir, "installed.json"))
	g := git.New()
	sync := New(store, paths, home, g)

	// Setup a package record in DB
	pkgName := "my-app"
	targetRel := "~/my-app-target"
	targetAbs := filepath.Join(home, "my-app-target")
	repoDir := filepath.Join(paths.RepoDir, pkgName)

	rec := storage.Record{
		Name:   pkgName,
		Repo:   "https://example.com/repo",
		Source: "my-config-file",
		Target: targetRel,
	}
	if err := store.Add(rec); err != nil {
		t.Fatalf("db save: %v", err)
	}

	// Case 1: Missing symlink and missing repo
	probs, err := sync.Check()
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if len(probs) != 1 || probs[0].Kind != ProblemMissingSymlink {
		t.Fatalf("Expected ProblemMissingSymlink, got: %+v", probs)
	}

	// Create backing git repository
	setupLocalGitRepo(t, repoDir)

	// Case 2: Still missing symlink, but repo is present
	probs, err = sync.Check()
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if len(probs) != 1 || probs[0].Kind != ProblemMissingSymlink {
		t.Fatalf("Expected ProblemMissingSymlink, got: %+v", probs)
	}

	// Create symlink
	sourceFile := filepath.Join(repoDir, "my-config-file")
	if err := os.Symlink(sourceFile, targetAbs); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	// Case 3: Happy path (no problems)
	probs, err = sync.Check()
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if len(probs) != 0 {
		t.Fatalf("Expected no problems, got: %+v", probs)
	}

	// Case 4: Broken symlink (source file deleted)
	_ = os.Remove(sourceFile)
	probs, err = sync.Check()
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if len(probs) != 1 || probs[0].Kind != ProblemBrokenSymlink {
		t.Fatalf("Expected ProblemBrokenSymlink, got: %+v", probs)
	}
}

func TestSyncerRepair(t *testing.T) {
	tempDir := t.TempDir()
	paths := config.Paths{
		ConfigDir: filepath.Join(tempDir, "config"),
		DataDir:   filepath.Join(tempDir, "data"),
		RepoDir:   filepath.Join(tempDir, "data", "repos"),
	}
	home := filepath.Join(tempDir, "home")
	_ = os.MkdirAll(home, 0o755)

	store := storage.New(filepath.Join(paths.DataDir, "installed.json"))
	g := git.New()
	sync := New(store, paths, home, g)

	pkgName := "my-app"
	targetRel := "~/my-app-target"
	targetAbs := filepath.Join(home, "my-app-target")
	repoDir := filepath.Join(paths.RepoDir, pkgName)

	rec := storage.Record{
		Name:   pkgName,
		Repo:   "https://example.com/repo",
		Source: "my-config-file",
		Target: targetRel,
	}
	setupLocalGitRepo(t, repoDir)

	// Test 1: Repair missing symlink
	if err := sync.Repair(rec); err != nil {
		t.Fatalf("Repair failed: %v", err)
	}

	// Verify symlink exists and resolves
	info, err := os.Lstat(targetAbs)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("expected symlink to be created by repair")
	}

	// Test 2: Repair broken symlink
	// break the symlink manually by deleting the file but keep the symlink
	_ = os.Remove(filepath.Join(repoDir, "my-config-file"))
	// recreate it as empty to make the source valid for repair
	_ = os.WriteFile(filepath.Join(repoDir, "my-config-file"), []byte("hi"), 0o644)
	
	if err := sync.Repair(rec); err != nil {
		t.Fatalf("Repair failed on broken symlink: %v", err)
	}

	// Test 3: Refuse to repair if a real user file is at target
	_ = os.Remove(targetAbs)
	if err := os.WriteFile(targetAbs, []byte("real config"), 0o644); err != nil {
		t.Fatalf("create real file: %v", err)
	}

	err = sync.Repair(rec)
	if err == nil {
		t.Error("expected Repair to fail when a real file is at target")
	}

	// Test 4: Refuse to repair if symlink points to a different source
	_ = os.Remove(targetAbs)
	otherDir := filepath.Join(tempDir, "other-dir")
	_ = os.MkdirAll(otherDir, 0o755)
	if err := os.Symlink(otherDir, targetAbs); err != nil {
		t.Fatalf("create other symlink: %v", err)
	}

	err = sync.Repair(rec)
	if err == nil {
		t.Error("expected Repair to fail when target points to a different source")
	}
}
