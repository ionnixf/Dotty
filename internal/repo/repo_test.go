package repo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ion/dotty/internal/catalog"
	"github.com/ion/dotty/internal/config"
)

// writeCatalog writes a packages.json into dir and returns dir's path.
func writeCatalog(t *testing.T, dir string, pkgs []catalog.Package) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	enc, err := os.Create(filepath.Join(dir, "packages.json"))
	if err != nil {
		t.Fatalf("create catalog: %v", err)
	}
	defer enc.Close()
	// Hand-encode so we don't depend on json import discipline in the test.
	enc.WriteString("[")
	for i, p := range pkgs {
		if i > 0 {
			enc.WriteString(",")
		}
		enc.WriteString(`{"name":"` + p.Name + `","repo":"` + p.Repo + `","target":"` + p.Target + `"`)
		if p.Source != "" {
			enc.WriteString(`,"source":"` + p.Source + `"`)
		}
		enc.WriteString("}")
	}
	enc.WriteString("]")
	return dir
}

// newTestManager builds a manager backed by a temp registry file, bypassing
// the seed so tests control exactly which repositories are registered.
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	path := filepath.Join(t.TempDir(), "repositories.json")
	m := &Manager{path: path}
	return m
}

func TestManagerSeedsOfficialAndPersists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "repositories.json")

	m, err := NewManager(path)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	// The official repository must always be present.
	got := m.List()
	if len(got) != 1 || got[0].Name != OfficialRepoName || got[0].Kind != KindEmbedded {
		t.Fatalf("expected only official embedded repo, got %+v", got)
	}
	// It must have been persisted to disk so a fresh load sees it too.
	m2, err := NewManager(path)
	if err != nil {
		t.Fatalf("reload manager: %v", err)
	}
	if len(m2.List()) != 1 || m2.List()[0].Name != OfficialRepoName {
		t.Fatalf("official repo not persisted: %+v", m2.List())
	}
}

func TestManagerAddRemoveAndOfficialProtected(t *testing.T) {
	m := newTestManager(t)

	localDir := writeCatalog(t, filepath.Join(t.TempDir(), "myrepo"), []catalog.Package{
		{Name: "alpha", Repo: "https://e.test/a", Target: "~/.config/a"},
	})
	if _, err := m.Add("mine", localDir); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Duplicate name rejected.
	if _, err := m.Add("mine", localDir); err == nil {
		t.Fatal("expected error adding duplicate repository name")
	}

	// official cannot be removed.
	if err := m.Remove(OfficialRepoName); err == nil {
		t.Fatal("expected error removing official repository")
	}
	// removing an unknown repo errors.
	if err := m.Remove("nope"); err == nil {
		t.Fatal("expected error removing unknown repository")
	}
}

func TestResolverMergeAndFirstWins(t *testing.T) {
	m := newTestManager(t)

	// Two local repositories that BOTH define "nvim", plus a unique one each.
	first := writeCatalog(t, filepath.Join(t.TempDir(), "first"), []catalog.Package{
		{Name: "nvim", Repo: "https://e.test/first-nvim", Target: "~/.config/nvim"},
		{Name: "first-only", Repo: "https://e.test/fo", Target: "~/.config/fo"},
	})
	second := writeCatalog(t, filepath.Join(t.TempDir(), "second"), []catalog.Package{
		{Name: "nvim", Repo: "https://e.test/second-nvim", Target: "~/.config/nvim"},
		{Name: "second-only", Repo: "https://e.test/so", Target: "~/.config/so"},
	})

	if _, err := m.Add("first", first); err != nil {
		t.Fatalf("add first: %v", err)
	}
	if _, err := m.Add("second", second); err != nil {
		t.Fatalf("add second: %v", err)
	}

	r := NewResolver(m, nil, config.Paths{RepoDir: t.TempDir()})

	entries, err := r.Merge()
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	// Three unique names: nvim, first-only, second-only.
	if len(entries) != 3 {
		t.Fatalf("expected 3 merged entries, got %d: %+v", len(entries), entries)
	}

	// "nvim" must resolve to the FIRST registered repository (first-wins).
	e, ok, err := r.Resolve("nvim")
	if err != nil || !ok {
		t.Fatalf("Resolve nvim: ok=%v err=%v", ok, err)
	}
	if e.RepoName != "first" || e.Repo != "https://e.test/first-nvim" {
		t.Fatalf("expected nvim from first repo, got %+v", e)
	}

	// Unique packages resolve from their own repo.
	e2, ok, err := r.Resolve("second-only")
	if err != nil || !ok {
		t.Fatalf("Resolve second-only: ok=%v err=%v", ok, err)
	}
	if e2.RepoName != "second" {
		t.Fatalf("expected second-only from second repo, got %+v", e2)
	}

	// Unknown name resolves to ok=false, no error.
	if _, ok, err := r.Resolve("missing"); ok || err != nil {
		t.Fatalf("expected missing to be not-ok/no-err, got ok=%v err=%v", ok, err)
	}
}

func TestResolverAllSortedAndOfficialEmbedded(t *testing.T) {
	// A manager with only the official repo should serve the embedded catalog
	// with no disk or network access (git client is nil).
	dir := t.TempDir()
	m, err := NewManager(filepath.Join(dir, "repositories.json"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	r := NewResolver(m, nil, config.Paths{})

	pkgs, err := r.All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(pkgs) == 0 {
		t.Fatal("expected embedded official catalog packages, got none")
	}
	// All() must return packages sorted by name.
	for i := 1; i < len(pkgs); i++ {
		if pkgs[i-1].Name > pkgs[i].Name {
			t.Fatalf("All not sorted: %q before %q", pkgs[i-1].Name, pkgs[i].Name)
		}
	}
}

func TestResolverContinuesOnFetchFailure(t *testing.T) {
	m := newTestManager(t)

	good := writeCatalog(t, filepath.Join(t.TempDir(), "good"), []catalog.Package{
		{Name: "good-pkg", Repo: "https://e.test/g", Target: "~/.config/g"},
	})
	// Point a "bad" local repository at a path with no packages.json.
	badDir := t.TempDir()

	if _, err := m.Add("good", good); err != nil {
		t.Fatalf("add good: %v", err)
	}
	if _, err := m.Add("bad", badDir); err != nil {
		t.Fatalf("add bad: %v", err)
	}

	r := NewResolver(m, nil, config.Paths{RepoDir: t.TempDir()})
	entries, err := r.Merge()
	// The bad repo (empty dir => empty index) does not error; the good repo's
	// package must still be present. This documents that an empty repo is valid
	// and never blocks the merge.
	if err != nil {
		t.Fatalf("Merge with empty repo should not error, got: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "good-pkg" {
		t.Fatalf("expected only good-pkg, got %+v", entries)
	}
}
