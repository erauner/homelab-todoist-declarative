package reconcile

import (
	"context"
	"fmt"
	"sort"

	"github.com/erauner/homelab-todoist-declarative/internal/todoist/sync"
	"github.com/erauner/homelab-todoist-declarative/internal/todoist/v1"
)

type Snapshot struct {
	Projects []v1.Project
	Labels   []v1.Label
	Filters  []sync.Filter

	projectByName map[string]v1.Project
	labelByName   map[string]v1.Label
	filterByName  map[string]sync.Filter
	projectNameByID map[string]string
}

func FetchSnapshot(ctx context.Context, v1c *v1.Client, syncc *sync.Client) (*Snapshot, error) {
	projects, err := v1c.ListProjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	labels, err := v1c.ListLabels(ctx)
	if err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}
	filtersResp, err := syncc.Read(ctx, []string{"filters"})
	if err != nil {
		return nil, fmt.Errorf("sync read filters: %w", err)
	}
	var filters []sync.Filter
	for _, f := range filtersResp.Filters {
		if f.IsDeleted {
			continue
		}
		filters = append(filters, f)
	}

	s := &Snapshot{
		Projects: projects,
		Labels:   labels,
		Filters:  filters,
		projectByName:    map[string]v1.Project{},
		labelByName:      map[string]v1.Label{},
		filterByName:     map[string]sync.Filter{},
		projectNameByID:  map[string]string{},
	}

	for _, p := range projects {
		s.projectNameByID[p.ID] = p.Name
		if _, ok := s.projectByName[p.Name]; ok {
			return nil, fmt.Errorf("remote has duplicate project name %q; cannot reconcile by name", p.Name)
		}
		s.projectByName[p.Name] = p
	}
	for _, l := range labels {
		if _, ok := s.labelByName[l.Name]; ok {
			return nil, fmt.Errorf("remote has duplicate label name %q; cannot reconcile by name", l.Name)
		}
		s.labelByName[l.Name] = l
	}
	for _, f := range filters {
		if _, ok := s.filterByName[f.Name]; ok {
			return nil, fmt.Errorf("remote has duplicate filter name %q; cannot reconcile by name", f.Name)
		}
		s.filterByName[f.Name] = f
	}

	// Ensure stable snapshot ordering for debugging/JSON output.
	sort.Slice(s.Projects, func(i, j int) bool { return s.Projects[i].Name < s.Projects[j].Name })
	sort.Slice(s.Labels, func(i, j int) bool { return s.Labels[i].Name < s.Labels[j].Name })
	sort.Slice(s.Filters, func(i, j int) bool { return s.Filters[i].Name < s.Filters[j].Name })

	return s, nil
}

func (s *Snapshot) ProjectByName(name string) (v1.Project, bool) {
	p, ok := s.projectByName[name]
	return p, ok
}

func (s *Snapshot) LabelByName(name string) (v1.Label, bool) {
	l, ok := s.labelByName[name]
	return l, ok
}

func (s *Snapshot) FilterByName(name string) (sync.Filter, bool) {
	f, ok := s.filterByName[name]
	return f, ok
}

func (s *Snapshot) ProjectNameByID(id string) (string, bool) {
	name, ok := s.projectNameByID[id]
	return name, ok
}
