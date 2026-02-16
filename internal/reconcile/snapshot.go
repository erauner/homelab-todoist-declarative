package reconcile

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/erauner/homelab-todoist-declarative/internal/todoist/sync"
	"github.com/erauner/homelab-todoist-declarative/internal/todoist/v1"
)

type Snapshot struct {
	Projects []v1.Project
	Labels   []v1.Label
	Filters  []sync.Filter
	Tasks    []v1.Task

	projectByName   map[string][]v1.Project
	projectByID     map[string]v1.Project
	labelByName     map[string][]v1.Label
	labelByID       map[string]v1.Label
	filterByName    map[string][]sync.Filter
	filterByID      map[string]sync.Filter
	taskByID        map[string]v1.Task
	taskByKey       map[string]v1.Task
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
	tasks, err := v1c.ListTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
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
		Projects:        projects,
		Labels:          labels,
		Filters:         filters,
		Tasks:           tasks,
		projectByName:   map[string][]v1.Project{},
		projectByID:     map[string]v1.Project{},
		labelByName:     map[string][]v1.Label{},
		labelByID:       map[string]v1.Label{},
		filterByName:    map[string][]sync.Filter{},
		filterByID:      map[string]sync.Filter{},
		taskByID:        map[string]v1.Task{},
		taskByKey:       map[string]v1.Task{},
		projectNameByID: map[string]string{},
	}

	for _, p := range projects {
		if _, ok := s.projectByID[p.ID]; ok {
			return nil, fmt.Errorf("remote has duplicate project id %q", p.ID)
		}
		s.projectByID[p.ID] = p
		s.projectNameByID[p.ID] = p.Name
		s.projectByName[p.Name] = append(s.projectByName[p.Name], p)
	}
	for _, l := range labels {
		if _, ok := s.labelByID[l.ID]; ok {
			return nil, fmt.Errorf("remote has duplicate label id %q", l.ID)
		}
		s.labelByID[l.ID] = l
		s.labelByName[l.Name] = append(s.labelByName[l.Name], l)
	}
	for _, f := range filters {
		if _, ok := s.filterByID[f.ID]; ok {
			return nil, fmt.Errorf("remote has duplicate filter id %q", f.ID)
		}
		s.filterByID[f.ID] = f
		s.filterByName[f.Name] = append(s.filterByName[f.Name], f)
	}
	for _, t := range tasks {
		if _, ok := s.taskByID[t.ID]; ok {
			return nil, fmt.Errorf("remote has duplicate task id %q", t.ID)
		}
		s.taskByID[t.ID] = t
		if key, ok := managedTaskKey(t.Description); ok {
			if _, exists := s.taskByKey[key]; exists {
				return nil, fmt.Errorf("remote has duplicate managed task key %q", key)
			}
			s.taskByKey[key] = t
		}
	}

	// Ensure stable snapshot ordering for debugging/JSON output.
	sort.Slice(s.Projects, func(i, j int) bool { return s.Projects[i].Name < s.Projects[j].Name })
	sort.Slice(s.Labels, func(i, j int) bool { return s.Labels[i].Name < s.Labels[j].Name })
	sort.Slice(s.Filters, func(i, j int) bool { return s.Filters[i].Name < s.Filters[j].Name })
	sort.Slice(s.Tasks, func(i, j int) bool { return s.Tasks[i].Content < s.Tasks[j].Content })

	return s, nil
}

func (s *Snapshot) ProjectByName(name string) (v1.Project, bool, error) {
	ps := s.projectByName[name]
	switch len(ps) {
	case 0:
		return v1.Project{}, false, nil
	case 1:
		return ps[0], true, nil
	default:
		ids := make([]string, 0, len(ps))
		for _, p := range ps {
			ids = append(ids, p.ID)
		}
		sort.Strings(ids)
		return v1.Project{}, false, fmt.Errorf("remote has %d projects named %q (ids: %s); cannot reconcile by name", len(ps), name, strings.Join(ids, ", "))
	}
}

func (s *Snapshot) ProjectByID(id string) (v1.Project, bool) {
	p, ok := s.projectByID[id]
	return p, ok
}

func (s *Snapshot) LabelByName(name string) (v1.Label, bool, error) {
	ls := s.labelByName[name]
	switch len(ls) {
	case 0:
		return v1.Label{}, false, nil
	case 1:
		return ls[0], true, nil
	default:
		ids := make([]string, 0, len(ls))
		for _, l := range ls {
			ids = append(ids, l.ID)
		}
		sort.Strings(ids)
		return v1.Label{}, false, fmt.Errorf("remote has %d labels named %q (ids: %s); cannot reconcile by name", len(ls), name, strings.Join(ids, ", "))
	}
}

func (s *Snapshot) LabelByID(id string) (v1.Label, bool) {
	l, ok := s.labelByID[id]
	return l, ok
}

func (s *Snapshot) FilterByName(name string) (sync.Filter, bool, error) {
	fs := s.filterByName[name]
	switch len(fs) {
	case 0:
		return sync.Filter{}, false, nil
	case 1:
		return fs[0], true, nil
	default:
		ids := make([]string, 0, len(fs))
		for _, f := range fs {
			ids = append(ids, f.ID)
		}
		sort.Strings(ids)
		return sync.Filter{}, false, fmt.Errorf("remote has %d filters named %q (ids: %s); cannot reconcile by name", len(fs), name, strings.Join(ids, ", "))
	}
}

func (s *Snapshot) FilterByID(id string) (sync.Filter, bool) {
	f, ok := s.filterByID[id]
	return f, ok
}

func (s *Snapshot) TaskByID(id string) (v1.Task, bool) {
	t, ok := s.taskByID[id]
	return t, ok
}

func (s *Snapshot) TaskByKey(key string) (v1.Task, bool) {
	t, ok := s.taskByKey[key]
	return t, ok
}

func (s *Snapshot) ProjectNameByID(id string) (string, bool) {
	name, ok := s.projectNameByID[id]
	return name, ok
}

func managedTaskKey(description string) (string, bool) {
	for _, ln := range strings.Split(description, "\n") {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, managedTaskKeyPrefix) {
			k := strings.TrimSpace(strings.TrimPrefix(ln, managedTaskKeyPrefix))
			if k != "" {
				return k, true
			}
		}
	}
	return "", false
}
