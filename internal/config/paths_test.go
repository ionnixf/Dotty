package config

import (
	"path/filepath"
	"testing"
)

func TestExpandInHome(t *testing.T) {
	home := t.TempDir()

	got, err := ExpandInHome("~/.config/nvim", home)
	if err != nil {
		t.Fatalf("ExpandInHome inside home: %v", err)
	}
	if want := filepath.Join(home, ".config", "nvim"); got != want {
		t.Fatalf("ExpandInHome = %q, want %q", got, want)
	}

	for _, path := range []string{"relative", "../outside", "/tmp/outside"} {
		if _, err := ExpandInHome(path, home); err == nil {
			t.Errorf("ExpandInHome(%q) succeeded outside home", path)
		}
	}
}
