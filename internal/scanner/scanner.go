// Package scanner looks for existing configuration directories under the
// user's home that Dotty could begin tracking, enabling the "Import Existing"
// feature. It only ever reads metadata; it never moves or modifies files.
package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Find is a discovered candidate for import: a directory that exists at one
// of the well-known target locations Dotty would otherwise create a symlink
// at, and that is not already a symlink Dotty manages.
type Find struct {
	Name   string // human-friendly name, e.g. "Neovim"
	Target string // absolute path, e.g. /home/user/.config/nvim
}

// KnownConfigs maps a config directory name to a friendly display name. The
// keys are the directory names scanned for under ~/.config. Adding a new
// importable config is a one-line change here.
func KnownConfigs() map[string]string {
	return map[string]string{
		"nvim":      "Neovim",
		"hypr":      "Hyprland",
		"waybar":    "Waybar",
		"foot":      "Foot",
		"fastfetch": "Fastfetch",
		"zsh":       "Zsh",
	}
}

// Scan walks ~/.config looking for the known directories. It returns finds
// sorted by display name. Symlinks are excluded so we do not propose
// re-importing something Dotty already linked.
func Scan(configDir string) ([]Find, error) {
	if configDir == "" {
		return nil, fmt.Errorf("empty config directory")
	}
	if _, err := os.Stat(configDir); err != nil {
		if os.IsNotExist(err) {
			return []Find{}, nil
		}
		return nil, fmt.Errorf("scan %s: %w", configDir, err)
	}

	var finds []Find
	for dir, name := range KnownConfigs() {
		target := filepath.Join(configDir, dir)
		info, err := os.Lstat(target)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("stat %s: %w", target, err)
			}
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue // already a symlink, likely Dotty-managed
		}
		if !info.IsDir() {
			continue // a file with this name; ignore
		}
		finds = append(finds, Find{Name: name, Target: target})
	}

	sort.Slice(finds, func(i, j int) bool { return finds[i].Name < finds[j].Name })
	return finds, nil
}
