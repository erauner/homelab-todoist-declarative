package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Example(t *testing.T) {
	path := filepath.Join("..", "..", "examples", "todoist.yaml")
	if _, err := Load(path); err != nil {
		t.Fatalf("Load(%q): %v", path, err)
	}
}

func TestNormalize_DefaultFilterOrder(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.yaml")
	// This is the preferred "simple" config wire format (no apiVersion/kind/metadata envelope).
	if err := os.WriteFile(p, []byte(`
name: t
projects: []
labels: []
prune:
  projects: false
  labels: false
  filters: false
filters:
  - name: A
    query: "today"
  - name: B
    query: "overdue"
`), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Spec.Filters[0].Order == nil || *cfg.Spec.Filters[0].Order != 1 {
		t.Fatalf("expected filter[0].order=1, got %#v", cfg.Spec.Filters[0].Order)
	}
	if cfg.Spec.Filters[1].Order == nil || *cfg.Spec.Filters[1].Order != 2 {
		t.Fatalf("expected filter[1].order=2, got %#v", cfg.Spec.Filters[1].Order)
	}
}
