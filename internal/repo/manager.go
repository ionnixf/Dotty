package repo

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ion/dotty/pkg/configs"
)

// OfficialRepoName is the name of the always-present seed repository backed by
// the embedded catalog. It is created on first load and never auto-removed.
const OfficialRepoName = "official"

// registry is the on-disk shape of the repositories file.
type registry struct {
	Repositories []Repository `json:"repositories"`
}

// Manager owns the persisted list of repositories. It is safe for use by a
// single goroutine, which matches Dotty's TUI (all state changes happen in
// tea.Update). All mutations are persisted immediately to path.
type Manager struct {
	path string
	repo []Repository
}

// NewManager loads the registry from path, seeding the official repository if
// the file is missing or does not yet contain it. The seeded official repo
// uses a sentinel URL the resolver recognises as "use the embedded catalog".
func NewManager(path string) (*Manager, error) {
	m := &Manager{path: path}
	if err := m.load(); err != nil {
		return nil, err
	}
	m.seedOfficial()
	return m, nil
}

// load reads the registry file. A missing file is treated as an empty
// registry, not an error.
func (m *Manager) load() error {
	raw, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			m.repo = nil
			return nil
		}
		return fmt.Errorf("read repositories %s: %w", m.path, err)
	}
	var reg registry
	if err := json.Unmarshal(raw, &reg); err != nil {
		return fmt.Errorf("parse repositories %s: %w", m.path, err)
	}
	m.repo = reg.Repositories
	return nil
}

// save writes the registry atomically via temp-file + rename.
func (m *Manager) save() error {
	raw, err := json.MarshalIndent(registry{Repositories: m.repo}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode repositories: %w", err)
	}
	raw = append(raw, '\n')

	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".repositories-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp repositories: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write repositories: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close repositories: %w", err)
	}
	if err := os.Rename(tmpName, m.path); err != nil {
		return fmt.Errorf("commit repositories: %w", err)
	}
	return nil
}

// seedOfficial ensures the embedded-catalog repository is present, once. It is
// marked with the sentinel URL so Fetch can short-circuit cloning.
func (m *Manager) seedOfficial() {
	for _, r := range m.repo {
		if r.Name == OfficialRepoName {
			return
		}
	}
	m.repo = append(m.repo, Repository{
		Name: OfficialRepoName,
		URL:  SentinelEmbedded,
		Kind: KindEmbedded,
	})
	// Best-effort persist; a failure here leaves the in-memory seed intact and
	// is retried on the next Add/Remove.
	_ = m.save()
}

// KindEmbedded marks the official repository whose index is the embedded
// catalog; it never needs fetching from disk or the network.
const KindEmbedded Kind = "embedded"

// SentinelEmbedded is the URL stored for the official repository; the resolver
// recognises it and serves the embedded packages.json directly.
const SentinelEmbedded = "embedded://catalog"

// List returns the repositories in registration order.
func (m *Manager) List() []Repository {
	out := make([]Repository, len(m.repo))
	copy(out, m.repo)
	return out
}

// Add registers a new repository by URL, inferring its kind. Returns an error
// if a repository with the same name already exists.
func (m *Manager) Add(name, url string) (Repository, error) {
	if name == "" {
		return Repository{}, errors.New("empty repository name")
	}
	for _, r := range m.repo {
		if r.Name == name {
			return Repository{}, fmt.Errorf("repository %q already exists", name)
		}
	}
	kind, err := inferKind(url)
	if err != nil {
		return Repository{}, err
	}
	rep := Repository{Name: name, URL: url, Kind: kind}
	m.repo = append(m.repo, rep)
	if err := m.save(); err != nil {
		// Roll back the in-memory add so caller state stays consistent.
		m.repo = m.repo[:len(m.repo)-1]
		return Repository{}, err
	}
	return rep, nil
}

// Remove unregisters a repository by name. The official repository cannot be
// removed; attempting to returns an error rather than silently failing.
func (m *Manager) Remove(name string) error {
	if name == OfficialRepoName {
		return fmt.Errorf("cannot remove the %q repository", OfficialRepoName)
	}
	idx := -1
	for i, r := range m.repo {
		if r.Name == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("repository %q not found", name)
	}
	removed := m.repo[idx]
	m.repo = append(m.repo[:idx], m.repo[idx+1:]...)
	if err := m.save(); err != nil {
		// Roll back the in-memory deletion
		m.repo = append(m.repo[:idx], append([]Repository{removed}, m.repo[idx:]...)...)
		return err
	}
	return nil
}

// EmbeddedPackagesJSON returns the official catalog bytes. Exported for the
// resolver, which uses it to build the official index without git or disk IO.
func EmbeddedPackagesJSON() []byte { return configs.PackagesJSON }
