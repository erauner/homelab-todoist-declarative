package reconcile

import (
	"testing"

	"github.com/erauner/homelab-todoist-declarative/internal/config"
	"github.com/erauner/homelab-todoist-declarative/internal/todoist/sync"
	"github.com/erauner/homelab-todoist-declarative/internal/todoist/v1"
)

func TestBuildPlan_CreateUpdateNoop(t *testing.T) {
	cfg := &config.TodoistConfig{
		APIVersion: config.APIVersion,
		Kind:       config.Kind,
		Metadata:   config.Metadata{Name: "test"},
		Spec: config.Spec{
			Projects: []config.ProjectSpec{
				{Name: "Work", Color: strPtr("red")},
				{Name: "Child", Parent: strPtr("Work")},
			},
			Labels: []config.LabelSpec{
				{Name: "waiting", IsFavorite: boolPtr(true)},
			},
			Filters: []config.FilterSpec{
				{Name: "Important", Query: "priority 1", Order: intPtr(1)},
			},
		},
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	remoteWorkParent := (*string)(nil)
	snap := &Snapshot{
		Projects: []v1.Project{
			{ID: "P1", Name: "Work", Color: "blue", IsFavorite: false, ViewStyle: "list", ParentID: remoteWorkParent, InboxProject: false},
			{ID: "P2", Name: "Child", Color: "green", IsFavorite: false, ViewStyle: "list", ParentID: nil, InboxProject: false},
		},
		Labels: []v1.Label{
			{ID: "L1", Name: "waiting", Color: "grey", IsFavorite: false},
		},
		Filters: []sync.Filter{
			{ID: "F1", Name: "Important", Query: "priority 1", Color: "red", ItemOrder: 1, IsFavorite: false, IsDeleted: false},
		},
		projectByName: map[string]v1.Project{},
		labelByName:   map[string]v1.Label{},
		filterByName:  map[string]sync.Filter{},
		projectNameByID: map[string]string{},
	}
	for _, p := range snap.Projects {
		snap.projectByName[p.Name] = p
		snap.projectNameByID[p.ID] = p.Name
	}
	for _, l := range snap.Labels {
		snap.labelByName[l.Name] = l
	}
	for _, f := range snap.Filters {
		snap.filterByName[f.Name] = f
	}

	plan, err := BuildPlan(cfg, snap, Options{Prune: false})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}

	// Expect:
	// - project Work update (color)
	// - project Child move (parent)
	// - label waiting update (is_favorite)
	// Filter is noop.
	if plan.Summary.Create != 0 {
		t.Fatalf("expected 0 creates, got %d", plan.Summary.Create)
	}
	if plan.Summary.Update != 2 {
		t.Fatalf("expected 2 updates, got %d", plan.Summary.Update)
	}
	if plan.Summary.Move != 1 {
		t.Fatalf("expected 1 move, got %d", plan.Summary.Move)
	}
	if plan.Summary.Delete != 0 {
		t.Fatalf("expected 0 deletes, got %d", plan.Summary.Delete)
	}
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool   { return &b }
func intPtr(i int) *int     { return &i }
