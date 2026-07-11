package catalog

import "testing"

func TestParseNormalisesLegacyAlternatives(t *testing.T) {
	pkgs, err := Parse([]byte(`[
  {"name":"nvim","target":"~/.config/nvim","alternatives":[
    {"name":"stable","repo":"https://example.test/stable"},
    {"name":"draft","repo":"https://example.test/draft","status":"placeholder"}
  ]}
]`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(pkgs) != 1 || len(pkgs[0].Configs) != 2 {
		t.Fatalf("unexpected configs: %+v", pkgs)
	}
	available := pkgs[0].AvailableConfigs()
	if len(available) != 1 || available[0].Name != "stable" {
		t.Fatalf("available configs = %+v, want only stable", available)
	}
}

func TestParseLegacyPlaceholderIsUnavailable(t *testing.T) {
	pkgs, err := Parse([]byte(`[
  {"name":"fish","repo":"https://example.test/fish","target":"~/.config/fish","placeholder":true}
]`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(pkgs[0].AvailableConfigs()) != 0 {
		t.Fatal("placeholder package must not have installable configs")
	}
}
