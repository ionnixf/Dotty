package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Settings is the persisted user configuration. JSON field tags keep the
// on-disk format stable and human-readable.
type Settings struct {
	RepoDirectory string `json:"repo_directory"`
	AutoUpdate    bool   `json:"auto_update"`
	ConfirmRemove bool   `json:"confirm_remove"`
	Theme         string `json:"theme"`
}

// DefaultSettings returns sensible initial settings. RepoDirectory uses
// the empty string as a sentinel meaning "use the default RepoDir from Paths".
func DefaultSettings() Settings {
	return Settings{
		RepoDirectory: "",
		AutoUpdate:    false,
		ConfirmRemove: true,
		Theme:         "dark",
	}
}

// Validate checks that a theme name is recognized. Unknown values are
// normalised to "dark" by the caller via NormaliseTheme rather than rejected,
// so a manually corrupted config never blocks startup.
func ValidTheme(theme string) bool {
	switch theme {
	case "dark", "light":
		return true
	}
	return false
}

// NormaliseTheme clamps an arbitrary theme string to a supported value.
func NormaliseTheme(theme string) string {
	if !ValidTheme(theme) {
		return "dark"
	}
	return theme
}

// LoadSettings reads settings from disk. A missing file yields the defaults
// rather than an error, so Dotty runs out of the box.
func LoadSettings(paths Paths) (Settings, error) {
	raw, err := os.ReadFile(paths.ConfigFile())
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultSettings(), nil
		}
		return Settings{}, fmt.Errorf("read config %s: %w", paths.ConfigFile(), err)
	}

	var s Settings
	if err := json.Unmarshal(raw, &s); err != nil {
		return Settings{}, fmt.Errorf("parse config %s: %w", paths.ConfigFile(), err)
	}
	s.Theme = NormaliseTheme(s.Theme)
	return s, nil
}

// SaveSettings writes settings to disk atomically. It writes to a sibling
// temp file and renames, so a crash mid-write cannot leave a truncated file.
func SaveSettings(paths Paths, s Settings) error {
	s.Theme = NormaliseTheme(s.Theme)

	if err := os.MkdirAll(paths.ConfigDir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	raw = append(raw, '\n')

	tmp, err := os.CreateTemp(paths.ConfigDir, ".config-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op after successful rename

	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close config: %w", err)
	}
	if err := os.Rename(tmpName, paths.ConfigFile()); err != nil {
		return fmt.Errorf("commit config: %w", err)
	}
	return nil
}

// ResolveRepoDir returns the absolute repository directory chosen by the user,
// falling back to the default Paths.RepoDir when the setting is empty.
func ResolveRepoDir(s Settings, paths Paths) string {
	if s.RepoDirectory == "" {
		return paths.RepoDir
	}
	return s.RepoDirectory
}
