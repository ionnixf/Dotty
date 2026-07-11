// Package catalog loads the list of packages Dotty knows how to install.
// The default catalog is embedded in the binary via pkg/configs; a user may
// override it by placing a packages.json in Dotty's config directory.
package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/ion/dotty/pkg/configs"
)

// Configuration is one installable configuration for an application.
// Status is "verified" or "placeholder"; placeholders remain documented in
// the index but are never offered for installation.
type Configuration struct {
	Name        string `json:"name"`
	Repo        string `json:"repo"`
	Source      string `json:"source,omitempty"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
}

// ConfigAlternative is retained for decoding legacy catalogs. New catalogs use
// configs, and parse normalises alternatives into Configurations.
type ConfigAlternative = Configuration

// Package is one installable entry in the catalog.
//
// Source is optional: when empty, the whole repository root is linked to
// Target ("target -> repo contents"). When set, only the repo/Source
// subdirectory is linked. This lets a single catalog describe both repos that
// keep their config at the root and repos that keep it under a subdirectory.
//
// If Alternatives is non-empty, the user is prompted to select which
// configuration they want to install.
type Package struct {
	Name         string              `json:"name"`
	Repo         string              `json:"repo,omitempty"`
	Source       string              `json:"source,omitempty"`
	Target       string              `json:"target"`
	Description  string              `json:"description,omitempty"`
	Alternatives []ConfigAlternative `json:"alternatives,omitempty"`
	Configs      []Configuration     `json:"configs,omitempty"`
	Status       string              `json:"status,omitempty"`
	Placeholder  bool                `json:"placeholder,omitempty"`
}

// AvailableConfigs returns the verified configurations for this application.
func (p Package) AvailableConfigs() []Configuration {
	configs := make([]Configuration, 0, len(p.Configs))
	for _, c := range p.Configs {
		if c.Status != "placeholder" {
			configs = append(configs, c)
		}
	}
	return configs
}

// Load returns the effective catalog: the user override if present, otherwise
// the embedded default. The result is sorted by name for stable display.
func Load(overridePath string) ([]Package, error) {
	if overridePath != "" {
		if raw, err := os.ReadFile(overridePath); err == nil {
			pkgs, perr := parse(raw)
			if perr != nil {
				return nil, fmt.Errorf("parse override catalog %s: %w", overridePath, perr)
			}
			return sorted(pkgs), nil
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read override catalog %s: %w", overridePath, err)
		}
	}

	pkgs, err := parse(configs.PackagesJSON)
	if err != nil {
		return nil, fmt.Errorf("parse embedded catalog: %w", err)
	}
	return sorted(pkgs), nil
}

// Parse decodes raw JSON into a validated Package slice. Exported so the
// repository layer can parse a fetched repository's packages.json using the
// exact same rules as the embedded catalog, without duplicating logic.
func Parse(raw []byte) ([]Package, error) { return parse(raw) }

// parse decodes raw JSON into a validated Package slice.
func parse(raw []byte) ([]Package, error) {
	var pkgs []Package
	if err := json.Unmarshal(raw, &pkgs); err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(pkgs))
	for i, p := range pkgs {
		if p.Name == "" {
			return nil, fmt.Errorf("package at index %d: empty name", i)
		}
		if p.Target == "" {
			return nil, fmt.Errorf("package %q: empty target", p.Name)
		}
		if err := normaliseConfigs(&p); err != nil {
			return nil, fmt.Errorf("package %q: %w", p.Name, err)
		}
		if seen[p.Name] {
			return nil, fmt.Errorf("duplicate package name %q", p.Name)
		}
		seen[p.Name] = true
		pkgs[i] = p
	}
	if pkgs == nil {
		pkgs = []Package{}
	}
	return pkgs, nil
}

func normaliseConfigs(p *Package) error {
	if p.Placeholder && p.Status == "" {
		p.Status = "placeholder"
	}
	if p.Status == "" {
		p.Status = "verified"
	}
	if p.Status != "verified" && p.Status != "placeholder" {
		return fmt.Errorf("unknown status %q", p.Status)
	}
	if len(p.Configs) == 0 {
		if p.Repo != "" {
			p.Configs = append(p.Configs, Configuration{
				Name: "default", Repo: p.Repo, Source: p.Source, Description: p.Description, Status: p.Status,
			})
		}
		p.Configs = append(p.Configs, p.Alternatives...)
	}
	if len(p.Configs) == 0 {
		return fmt.Errorf("no configurations")
	}
	seen := make(map[string]bool, len(p.Configs))
	for i := range p.Configs {
		c := &p.Configs[i]
		if c.Name == "" {
			return fmt.Errorf("configuration at index %d: empty name", i)
		}
		if c.Repo == "" {
			return fmt.Errorf("configuration %q: empty repo", c.Name)
		}
		if c.Status == "" {
			c.Status = p.Status
		}
		if c.Status != "verified" && c.Status != "placeholder" {
			return fmt.Errorf("configuration %q: unknown status %q", c.Name, c.Status)
		}
		if seen[c.Name] {
			return fmt.Errorf("duplicate configuration name %q", c.Name)
		}
		seen[c.Name] = true
	}
	return nil
}

// Sorted returns pkgs sorted by name. Exported for the repository layer.
func Sorted(pkgs []Package) []Package { return sorted(pkgs) }

func sorted(pkgs []Package) []Package {
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs
}

// Find returns the package with the given name from pkgs.
func Find(pkgs []Package, name string) (Package, bool) {
	for _, p := range pkgs {
		if p.Name == name {
			return p, true
		}
	}
	return Package{}, false
}
