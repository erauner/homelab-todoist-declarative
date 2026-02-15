package export

import (
	"testing"

	"github.com/erauner/homelab-todoist-declarative/internal/reconcile"
	"github.com/erauner/homelab-todoist-declarative/internal/todoist/sync"
	"github.com/erauner/homelab-todoist-declarative/internal/todoist/v1"
)

func TestFromSnapshot_Minimal(t *testing.T) {
	parentID := "P0"
	snap := &reconcile.Snapshot{
		Projects: []v1.Project{
			{ID: "P0", Name: "Root", InboxProject: false},
			{ID: "P1", Name: "Child", ParentID: &parentID, InboxProject: false},
		},
		Labels: []v1.Label{
			{ID: "L1", Name: "waiting", Color: "grey", IsFavorite: true},
		},
		Filters: []sync.Filter{
			{ID: "F1", Name: "Focus", Query: "today", Color: "red", ItemOrder: 3, IsFavorite: true, IsDeleted: false},
		},
	}
	// ProjectNameByID relies on the index; simulate a post-FetchSnapshot snapshot.
	// (export should still behave reasonably even if an ID lookup fails.)
	snap2 := *snap
	snap2.Projects = append([]v1.Project(nil), snap.Projects...)
	// Minimal map for ProjectNameByID.
	// We don't need the duplicate-detection maps for this test.
	// This mirrors what FetchSnapshot builds.
	_ = snap2

	cfg, err := FromSnapshot("t", snap, Options{Full: false})
	if err != nil {
		t.Fatalf("FromSnapshot: %v", err)
	}
	if cfg.Name != "t" {
		t.Fatalf("expected name t, got %q", cfg.Name)
	}
	if len(cfg.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(cfg.Projects))
	}
	if cfg.Projects[0].Name == "" {
		t.Fatalf("expected project name set")
	}
	if len(cfg.Labels) != 1 || cfg.Labels[0].Name != "waiting" {
		t.Fatalf("unexpected labels: %#v", cfg.Labels)
	}
	if len(cfg.Filters) != 1 || cfg.Filters[0].Name != "Focus" || cfg.Filters[0].Query != "today" {
		t.Fatalf("unexpected filters: %#v", cfg.Filters)
	}
	if cfg.Filters[0].Order == nil || *cfg.Filters[0].Order != 3 {
		t.Fatalf("expected order=3, got %#v", cfg.Filters[0].Order)
	}
	// Minimal export should not set managed fields.
	if cfg.Labels[0].Color != nil || cfg.Labels[0].IsFavorite != nil {
		t.Fatalf("expected minimal label export to omit managed fields, got %#v", cfg.Labels[0])
	}
}

func TestFromSnapshot_Full(t *testing.T) {
	snap := &reconcile.Snapshot{
		Projects: []v1.Project{
			{ID: "P1", Name: "Work", Color: "red", IsFavorite: true, ViewStyle: "list", InboxProject: false},
		},
		Labels: []v1.Label{
			{ID: "L1", Name: "waiting", Color: "grey", IsFavorite: true},
		},
		Filters: []sync.Filter{
			{ID: "F1", Name: "Focus", Query: "today", Color: "red", ItemOrder: 1, IsFavorite: true, IsDeleted: false},
		},
	}

	cfg, err := FromSnapshot("t", snap, Options{Full: true})
	if err != nil {
		t.Fatalf("FromSnapshot: %v", err)
	}
	if cfg.Projects[0].Color == nil || *cfg.Projects[0].Color != "red" {
		t.Fatalf("expected project color red, got %#v", cfg.Projects[0].Color)
	}
	if cfg.Projects[0].IsFavorite == nil || *cfg.Projects[0].IsFavorite != true {
		t.Fatalf("expected project favorite true, got %#v", cfg.Projects[0].IsFavorite)
	}
	if cfg.Projects[0].ViewStyle == nil || *cfg.Projects[0].ViewStyle != "list" {
		t.Fatalf("expected view_style list, got %#v", cfg.Projects[0].ViewStyle)
	}
	if cfg.Labels[0].Color == nil || *cfg.Labels[0].Color != "grey" {
		t.Fatalf("expected label color grey, got %#v", cfg.Labels[0].Color)
	}
	if cfg.Labels[0].IsFavorite == nil || *cfg.Labels[0].IsFavorite != true {
		t.Fatalf("expected label favorite true, got %#v", cfg.Labels[0].IsFavorite)
	}
	if cfg.Filters[0].Color == nil || *cfg.Filters[0].Color != "red" {
		t.Fatalf("expected filter color red, got %#v", cfg.Filters[0].Color)
	}
	if cfg.Filters[0].IsFavorite == nil || *cfg.Filters[0].IsFavorite != true {
		t.Fatalf("expected filter favorite true, got %#v", cfg.Filters[0].IsFavorite)
	}
}
