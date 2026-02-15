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
		projectByName:   map[string][]v1.Project{},
		labelByName:     map[string][]v1.Label{},
		filterByName:    map[string][]sync.Filter{},
		projectNameByID: map[string]string{},
	}
	for _, p := range snap.Projects {
		snap.projectByName[p.Name] = append(snap.projectByName[p.Name], p)
		snap.projectNameByID[p.ID] = p.Name
	}
	for _, l := range snap.Labels {
		snap.labelByName[l.Name] = append(snap.labelByName[l.Name], l)
	}
	for _, f := range snap.Filters {
		snap.filterByName[f.Name] = append(snap.filterByName[f.Name], f)
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

func TestBuildPlan_ProjectID_DisambiguatesDuplicateName(t *testing.T) {
	cfg := &config.TodoistConfig{
		Metadata: config.Metadata{Name: "test"},
		Spec: config.Spec{
			Projects: []config.ProjectSpec{
				{ID: strPtr("P1"), Name: "Renamed", Color: strPtr("red")},
			},
		},
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	p1 := v1.Project{ID: "P1", Name: "Dup", Color: "blue", IsFavorite: false, ViewStyle: "list", ParentID: nil, InboxProject: false}
	p2 := v1.Project{ID: "P2", Name: "Dup", Color: "blue", IsFavorite: false, ViewStyle: "list", ParentID: nil, InboxProject: false}
	snap := &Snapshot{
		Projects:        []v1.Project{p1, p2},
		Labels:          nil,
		Filters:         nil,
		projectByName:   map[string][]v1.Project{"Dup": {p1, p2}},
		projectByID:     map[string]v1.Project{"P1": p1, "P2": p2},
		labelByName:     map[string][]v1.Label{},
		labelByID:       map[string]v1.Label{},
		filterByName:    map[string][]sync.Filter{},
		filterByID:      map[string]sync.Filter{},
		projectNameByID: map[string]string{"P1": "Dup", "P2": "Dup"},
	}

	plan, err := BuildPlan(cfg, snap, Options{Prune: false})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if plan.Summary.Update != 1 {
		t.Fatalf("expected 1 update, got %d", plan.Summary.Update)
	}
	found := false
	for _, op := range plan.Operations {
		if op.Kind == KindProject && op.Action == ActionUpdate && op.ID == "P1" && op.Name == "Renamed" {
			found = true
			if len(op.Changes) != 2 {
				t.Fatalf("expected 2 changes (name,color), got %#v", op.Changes)
			}
		}
	}
	if !found {
		t.Fatalf("expected project update op for P1")
	}
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }
func intPtr(i int) *int       { return &i }
