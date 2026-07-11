package installer

import (
	"errors"
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

func TestInstallerValidation(t *testing.T) {
	tempDir := t.TempDir()
	paths := config.Paths{
		ConfigDir: filepath.Join(tempDir, "config"),
		DataDir:   filepath.Join(tempDir, "data"),
		RepoDir:   filepath.Join(tempDir, "data", "repos"),
	}
	home := filepath.Join(tempDir, "home")
	_ = os.MkdirAll(home, 0o755)

	storeFile := filepath.Join(paths.DataDir, "installed.json")
	store := storage.New(storeFile)
	g := git.New()
	inst := New(g, store, paths, home)

	// 1. Path traversal in Name (req.Name)
	reqTraversal := Request{
		Name:   "../../bad-pkg",
		Repo:   "https://example.com/bad",
		Target: "~/config/bad",
	}
	_, err := inst.Install(reqTraversal)
	if err == nil {
		t.Error("expected error for path traversal in Name, got nil")
	}

	// 1b. Target outside home directory
	reqTargetOutside := Request{
		Name:   "good-pkg",
		Repo:   "https://example.com/good",
		Target: "/etc/shadow",
	}
	_, err = inst.Install(reqTargetOutside)
	if err == nil {
		t.Error("expected error for target outside home directory, got nil")
	}

	// 1c. Relative target path
	reqTargetRelative := Request{
		Name:   "good-pkg",
		Repo:   "https://example.com/good",
		Target: "relative/path",
	}
	_, err = inst.Install(reqTargetRelative)
	if err == nil {
		t.Error("expected error for relative target path, got nil")
	}

	// 2. LinkSource validation (req.Source)
	// Create a valid dummy repo directory first
	repoDir := filepath.Join(paths.RepoDir, "good-pkg")
	setupLocalGitRepo(t, repoDir)

	reqSourceTraversal := Request{
		Name:   "good-pkg",
		Repo:   repoDir, // Use local path as remote URL to allow clone to succeed
		Source: "../../../escaping-source",
		Target: "~/config/target",
	}
	_, err = inst.Install(reqSourceTraversal)
	if err == nil {
		t.Error("expected error for path traversal in Source, got nil")
	}
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); err != nil {
		t.Errorf("invalid source must not alter an existing repository: %v", err)
	}
}

func TestInstallerHappyPathAndSymlinkOverwriting(t *testing.T) {
	tempDir := t.TempDir()
	paths := config.Paths{
		ConfigDir: filepath.Join(tempDir, "config"),
		DataDir:   filepath.Join(tempDir, "data"),
		RepoDir:   filepath.Join(tempDir, "data", "repos"),
	}
	home := filepath.Join(tempDir, "home")
	_ = os.MkdirAll(home, 0o755)

	storeFile := filepath.Join(paths.DataDir, "installed.json")
	store := storage.New(storeFile)
	g := git.New()
	inst := New(g, store, paths, home)

	// Create a dummy remote repo
	remoteRepo := filepath.Join(tempDir, "remote-repo")
	setupLocalGitRepo(t, remoteRepo)

	req := Request{
		Name:   "my-pkg",
		Repo:   remoteRepo,
		Source: "", // link root
		Target: "~/my-target-config",
	}

	res, err := inst.Install(req)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	targetAbs := res.Target

	// Check symlink exists
	info, err := os.Lstat(targetAbs)
	if err != nil {
		t.Fatalf("expected symlink to exist: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected target to be a symlink")
	}

	// Check record is saved
	rec, ok, err := store.Find("my-pkg")
	if err != nil || !ok {
		t.Fatalf("expected package to be recorded in DB: ok=%v, err=%v", ok, err)
	}
	if rec.Target != req.Target {
		t.Errorf("DB target mismatch: got %q, want %q", rec.Target, req.Target)
	}

	// Re-installing the same package (idempotent, target symlink points to same source)
	_, err = inst.Install(req)
	if err != nil {
		t.Errorf("re-installation failed: %v", err)
	}

	// Try to install when target points to a DIFFERENT file (user owned)
	_ = os.Remove(targetAbs)
	otherDir := filepath.Join(tempDir, "other-dir")
	_ = os.MkdirAll(otherDir, 0o755)
	if err := os.Symlink(otherDir, targetAbs); err != nil {
		t.Fatalf("create user symlink: %v", err)
	}

	_, err = inst.Install(req)
	if err == nil || !errors.Is(err, ErrTargetExists) {
		t.Errorf("expected ErrTargetExists when target points to different source, got: %v", err)
	}
}

func TestInstallerRollback(t *testing.T) {
	tempDir := t.TempDir()
	paths := config.Paths{
		ConfigDir: filepath.Join(tempDir, "config"),
		DataDir:   filepath.Join(tempDir, "data"),
		RepoDir:   filepath.Join(tempDir, "data", "repos"),
	}
	home := filepath.Join(tempDir, "home")
	_ = os.MkdirAll(home, 0o755)

	// Use a path in a directory we cannot create/write to (like a root-level nonexistent dir)
	// to make database save fail.
	store := storage.New("/nonexistent_root_dir_for_test/installed.json")
	g := git.New()

	inst := New(g, store, paths, home)

	remoteRepo := filepath.Join(tempDir, "remote-repo")
	setupLocalGitRepo(t, remoteRepo)

	req := Request{
		Name:   "my-pkg",
		Repo:   remoteRepo,
		Source: "",
		Target: "~/my-target-config",
	}

	targetAbs := filepath.Join(home, "my-target-config")

	_, err := inst.Install(req)
	if err == nil {
		t.Fatal("expected Install to fail due to db save failure")
	}

	// Verify that the symlink was rolled back (deleted) because it was created by this call
	_, err = os.Lstat(targetAbs)
	if !os.IsNotExist(err) {
		t.Errorf("expected target symlink to be rolled back/deleted, but got error: %v", err)
	}

	// Now manually create the correct symlink
	repoDir := filepath.Join(paths.RepoDir, "my-pkg")
	source := LinkSourceOnly(repoDir, "")
	if err := os.Symlink(source, targetAbs); err != nil {
		t.Fatalf("manually create symlink: %v", err)
	}

	// Re-run Install (should fail on db save)
	_, err = inst.Install(req)
	if err == nil {
		t.Fatal("expected Install to fail due to db save failure")
	}

	// Verify that the pre-existing symlink was NOT deleted on rollback because it already existed
	_, err = os.Lstat(targetAbs)
	if err != nil {
		t.Errorf("expected pre-existing symlink to remain, but got error: %v", err)
	}
}

// Simple helper to mimic LinkSource for tests without error handling
func LinkSourceOnly(repoDir, source string) string {
	if source == "" {
		return repoDir
	}
	return filepath.Join(repoDir, source)
}
