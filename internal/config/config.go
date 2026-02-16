package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	APIVersion = "homelab.todoist/v1"
	Kind       = "TodoistConfig"
)

type TodoistConfig struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`
}

type Metadata struct {
	Name string `yaml:"name"`
}

type Spec struct {
	Projects []ProjectSpec `yaml:"projects"`
	Labels   []LabelSpec   `yaml:"labels"`
	Filters  []FilterSpec  `yaml:"filters"`
	Tasks    []TaskSpec    `yaml:"tasks"`
	Prune    PruneSpec     `yaml:"prune"`
}

type PruneSpec struct {
	Projects bool `yaml:"projects"`
	Labels   bool `yaml:"labels"`
	Filters  bool `yaml:"filters"`
	Tasks    bool `yaml:"tasks"`
}

type ProjectSpec struct {
	ID         *string `yaml:"id,omitempty"`
	Name       string  `yaml:"name"`
	Parent     *string `yaml:"parent,omitempty"`
	Color      *string `yaml:"color,omitempty"`
	IsFavorite *bool   `yaml:"is_favorite,omitempty"`
	ViewStyle  *string `yaml:"view_style,omitempty"`
}

type LabelSpec struct {
	ID         *string `yaml:"id,omitempty"`
	Name       string  `yaml:"name"`
	Color      *string `yaml:"color,omitempty"`
	IsFavorite *bool   `yaml:"is_favorite,omitempty"`
}

type FilterSpec struct {
	ID         *string `yaml:"id,omitempty"`
	Name       string  `yaml:"name"`
	Query      string  `yaml:"query"`
	Color      *string `yaml:"color,omitempty"`
	IsFavorite *bool   `yaml:"is_favorite,omitempty"`
	Order      *int    `yaml:"order,omitempty"`
}

type TaskDueSpec struct {
	String *string `yaml:"string,omitempty"`
}

type TaskSpec struct {
	ID          *string     `yaml:"id,omitempty"`
	Key         string      `yaml:"key,omitempty"`
	Type        *string     `yaml:"type,omitempty"` // recurring_template (MVP)
	Content     string      `yaml:"content"`
	Description *string     `yaml:"description,omitempty"`
	Project     *string     `yaml:"project,omitempty"` // project name
	Labels      []string    `yaml:"labels,omitempty"`
	Priority    *int        `yaml:"priority,omitempty"` // 1..4
	Due         TaskDueSpec `yaml:"due,omitempty"`
}

func Load(path string) (*TodoistConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	// Accept two wire formats:
	// 1) "envelope": {apiVersion, kind, metadata, spec} (old/default)
	// 2) "simple": {name, prune, projects, labels, filters}
	//
	// The simple format avoids Kubernetes-like conventions while keeping backwards compatibility.
	var top map[string]any
	if err := yaml.Unmarshal(b, &top); err != nil {
		return nil, fmt.Errorf("parse yaml %q: %w", path, err)
	}

	var cfg TodoistConfig
	if _, hasSpec := top["spec"]; hasSpec || top["apiVersion"] != nil || top["kind"] != nil || top["metadata"] != nil {
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return nil, fmt.Errorf("parse yaml %q: %w", path, err)
		}
	} else {
		var sc simpleConfig
		if err := yaml.Unmarshal(b, &sc); err != nil {
			return nil, fmt.Errorf("parse yaml %q: %w", path, err)
		}
		cfg = TodoistConfig{
			Metadata: Metadata{Name: sc.Name},
			Spec: Spec{
				Projects: sc.Projects,
				Labels:   sc.Labels,
				Filters:  sc.Filters,
				Tasks:    sc.Tasks,
				Prune:    sc.Prune,
			},
		}
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

type simpleConfig struct {
	Name     string        `yaml:"name"`
	Projects []ProjectSpec `yaml:"projects"`
	Labels   []LabelSpec   `yaml:"labels"`
	Filters  []FilterSpec  `yaml:"filters"`
	Tasks    []TaskSpec    `yaml:"tasks"`
	Prune    PruneSpec     `yaml:"prune"`
}

// DefaultPath returns the default config path used by the CLI.
func DefaultPath() string { return "todoist.yaml" }

func (c *TodoistConfig) Normalize() {
	c.APIVersion = strings.TrimSpace(c.APIVersion)
	c.Kind = strings.TrimSpace(c.Kind)
	c.Metadata.Name = strings.TrimSpace(c.Metadata.Name)

	// Normalize names/queries. We do *not* lowercase; identity keys are case-sensitive.
	for i := range c.Spec.Projects {
		c.Spec.Projects[i].Name = strings.TrimSpace(c.Spec.Projects[i].Name)
		if c.Spec.Projects[i].ID != nil {
			id := strings.TrimSpace(*c.Spec.Projects[i].ID)
			c.Spec.Projects[i].ID = &id
		}
		if c.Spec.Projects[i].Parent != nil {
			p := strings.TrimSpace(*c.Spec.Projects[i].Parent)
			c.Spec.Projects[i].Parent = &p
		}
		if c.Spec.Projects[i].ViewStyle != nil {
			vs := strings.TrimSpace(*c.Spec.Projects[i].ViewStyle)
			c.Spec.Projects[i].ViewStyle = &vs
		}
		if c.Spec.Projects[i].Color != nil {
			col := strings.TrimSpace(*c.Spec.Projects[i].Color)
			c.Spec.Projects[i].Color = &col
		}
	}
	for i := range c.Spec.Labels {
		c.Spec.Labels[i].Name = strings.TrimSpace(c.Spec.Labels[i].Name)
		if c.Spec.Labels[i].ID != nil {
			id := strings.TrimSpace(*c.Spec.Labels[i].ID)
			c.Spec.Labels[i].ID = &id
		}
		if c.Spec.Labels[i].Color != nil {
			col := strings.TrimSpace(*c.Spec.Labels[i].Color)
			c.Spec.Labels[i].Color = &col
		}
	}
	for i := range c.Spec.Filters {
		c.Spec.Filters[i].Name = strings.TrimSpace(c.Spec.Filters[i].Name)
		c.Spec.Filters[i].Query = strings.TrimSpace(c.Spec.Filters[i].Query)
		if c.Spec.Filters[i].ID != nil {
			id := strings.TrimSpace(*c.Spec.Filters[i].ID)
			c.Spec.Filters[i].ID = &id
		}
		if c.Spec.Filters[i].Color != nil {
			col := strings.TrimSpace(*c.Spec.Filters[i].Color)
			c.Spec.Filters[i].Color = &col
		}
		// Default order to list position (1-indexed) for determinism.
		if c.Spec.Filters[i].Order == nil {
			ord := i + 1
			c.Spec.Filters[i].Order = &ord
		}
	}
	for i := range c.Spec.Tasks {
		c.Spec.Tasks[i].Key = strings.TrimSpace(c.Spec.Tasks[i].Key)
		c.Spec.Tasks[i].Content = strings.TrimSpace(c.Spec.Tasks[i].Content)
		if c.Spec.Tasks[i].ID != nil {
			id := strings.TrimSpace(*c.Spec.Tasks[i].ID)
			c.Spec.Tasks[i].ID = &id
		}
		if c.Spec.Tasks[i].Type != nil {
			typ := strings.TrimSpace(*c.Spec.Tasks[i].Type)
			c.Spec.Tasks[i].Type = &typ
		}
		if c.Spec.Tasks[i].Description != nil {
			d := strings.TrimSpace(*c.Spec.Tasks[i].Description)
			c.Spec.Tasks[i].Description = &d
		}
		if c.Spec.Tasks[i].Project != nil {
			p := strings.TrimSpace(*c.Spec.Tasks[i].Project)
			c.Spec.Tasks[i].Project = &p
		}
		if c.Spec.Tasks[i].Due.String != nil {
			ds := strings.TrimSpace(*c.Spec.Tasks[i].Due.String)
			c.Spec.Tasks[i].Due.String = &ds
		}
		for j := range c.Spec.Tasks[i].Labels {
			c.Spec.Tasks[i].Labels[j] = strings.TrimSpace(c.Spec.Tasks[i].Labels[j])
		}
	}
}

func (c *TodoistConfig) Validate() error {
	var errs []error

	if c.APIVersion != "" && c.APIVersion != APIVersion {
		errs = append(errs, fmt.Errorf("apiVersion must be %q (got %q)", APIVersion, c.APIVersion))
	}
	if c.Kind != "" && c.Kind != Kind {
		errs = append(errs, fmt.Errorf("kind must be %q (got %q)", Kind, c.Kind))
	}
	if c.Metadata.Name == "" {
		errs = append(errs, errors.New("name is required"))
	}

	// Projects: names unique + parents exist + no cycles.
	projectNames := make(map[string]struct{}, len(c.Spec.Projects))
	projectIDs := make(map[string]struct{}, len(c.Spec.Projects))
	for i, p := range c.Spec.Projects {
		if p.Name == "" {
			errs = append(errs, fmt.Errorf("spec.projects[%d].name is required", i))
			continue
		}
		if _, ok := projectNames[p.Name]; ok {
			errs = append(errs, fmt.Errorf("duplicate project name %q", p.Name))
		} else {
			projectNames[p.Name] = struct{}{}
		}
		if p.ID != nil {
			if *p.ID == "" {
				errs = append(errs, fmt.Errorf("spec.projects[%d] (%q).id cannot be empty", i, p.Name))
			} else if _, ok := projectIDs[*p.ID]; ok {
				errs = append(errs, fmt.Errorf("duplicate project id %q", *p.ID))
			} else {
				projectIDs[*p.ID] = struct{}{}
			}
		}
		if p.Parent != nil {
			if *p.Parent == "" {
				errs = append(errs, fmt.Errorf("spec.projects[%d].parent cannot be empty string; omit to move to root", i))
			} else if _, ok := projectNames[*p.Parent]; !ok {
				// We validate parent existence after collecting all names, so skip for now.
			}
		}
	}
	for i, p := range c.Spec.Projects {
		if p.Parent != nil && *p.Parent != "" {
			if _, ok := projectNames[*p.Parent]; !ok {
				errs = append(errs, fmt.Errorf("spec.projects[%d] (%q) references unknown parent %q", i, p.Name, *p.Parent))
			}
		}
	}
	if err := validateProjectAcyclic(c.Spec.Projects); err != nil {
		errs = append(errs, err)
	}

	// Labels: names unique.
	labelNames := make(map[string]struct{}, len(c.Spec.Labels))
	labelIDs := make(map[string]struct{}, len(c.Spec.Labels))
	for i, l := range c.Spec.Labels {
		if l.Name == "" {
			errs = append(errs, fmt.Errorf("spec.labels[%d].name is required", i))
			continue
		}
		if _, ok := labelNames[l.Name]; ok {
			errs = append(errs, fmt.Errorf("duplicate label name %q", l.Name))
		} else {
			labelNames[l.Name] = struct{}{}
		}
		if l.ID != nil {
			if *l.ID == "" {
				errs = append(errs, fmt.Errorf("spec.labels[%d] (%q).id cannot be empty", i, l.Name))
			} else if _, ok := labelIDs[*l.ID]; ok {
				errs = append(errs, fmt.Errorf("duplicate label id %q", *l.ID))
			} else {
				labelIDs[*l.ID] = struct{}{}
			}
		}
	}

	// Filters: names unique, query required, order positive.
	filterNames := make(map[string]struct{}, len(c.Spec.Filters))
	filterIDs := make(map[string]struct{}, len(c.Spec.Filters))
	for i, f := range c.Spec.Filters {
		if f.Name == "" {
			errs = append(errs, fmt.Errorf("spec.filters[%d].name is required", i))
			continue
		}
		if _, ok := filterNames[f.Name]; ok {
			errs = append(errs, fmt.Errorf("duplicate filter name %q", f.Name))
		} else {
			filterNames[f.Name] = struct{}{}
		}
		if f.ID != nil {
			if *f.ID == "" {
				errs = append(errs, fmt.Errorf("spec.filters[%d] (%q).id cannot be empty", i, f.Name))
			} else if _, ok := filterIDs[*f.ID]; ok {
				errs = append(errs, fmt.Errorf("duplicate filter id %q", *f.ID))
			} else {
				filterIDs[*f.ID] = struct{}{}
			}
		}
		if f.Query == "" {
			errs = append(errs, fmt.Errorf("spec.filters[%d] (%q).query is required", i, f.Name))
		}
		if f.Order == nil || *f.Order <= 0 {
			errs = append(errs, fmt.Errorf("spec.filters[%d] (%q).order must be >= 1", i, f.Name))
		}
	}

	// Tasks: require stable identity (id or key) and validate recurring template constraints.
	taskIDs := make(map[string]struct{}, len(c.Spec.Tasks))
	taskKeys := make(map[string]struct{}, len(c.Spec.Tasks))
	for i, t := range c.Spec.Tasks {
		if t.Content == "" {
			errs = append(errs, fmt.Errorf("spec.tasks[%d].content is required", i))
		}
		if t.ID == nil && t.Key == "" {
			errs = append(errs, fmt.Errorf("spec.tasks[%d] requires either id or key", i))
		}
		if t.ID != nil {
			if *t.ID == "" {
				errs = append(errs, fmt.Errorf("spec.tasks[%d].id cannot be empty", i))
			} else if _, ok := taskIDs[*t.ID]; ok {
				errs = append(errs, fmt.Errorf("duplicate task id %q", *t.ID))
			} else {
				taskIDs[*t.ID] = struct{}{}
			}
		}
		if t.Key != "" {
			if _, ok := taskKeys[t.Key]; ok {
				errs = append(errs, fmt.Errorf("duplicate task key %q", t.Key))
			} else {
				taskKeys[t.Key] = struct{}{}
			}
		}
		if t.Type != nil && *t.Type != "" && *t.Type != "recurring_template" {
			errs = append(errs, fmt.Errorf("spec.tasks[%d] (%q).type must be recurring_template when set", i, t.Content))
		}
		if t.Type != nil && *t.Type == "recurring_template" && (t.Due.String == nil || *t.Due.String == "") {
			errs = append(errs, fmt.Errorf("spec.tasks[%d] (%q) recurring_template requires due.string", i, t.Content))
		}
		if t.Priority != nil && (*t.Priority < 1 || *t.Priority > 4) {
			errs = append(errs, fmt.Errorf("spec.tasks[%d] (%q).priority must be in [1,4]", i, t.Content))
		}
		for j, l := range t.Labels {
			if l == "" {
				errs = append(errs, fmt.Errorf("spec.tasks[%d].labels[%d] cannot be empty", i, j))
			}
		}
		if t.Due.String != nil && *t.Due.String == "" {
			errs = append(errs, fmt.Errorf("spec.tasks[%d] (%q).due.string cannot be empty when set", i, t.Content))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func validateProjectAcyclic(projects []ProjectSpec) error {
	// Graph: child -> parent
	parent := make(map[string]string, len(projects))
	for _, p := range projects {
		if p.Parent != nil && *p.Parent != "" {
			parent[p.Name] = *p.Parent
		}
	}

	// DFS cycle detection.
	type state int
	const (
		unvisited state = iota
		visiting
		visited
	)
	st := make(map[string]state, len(projects))
	var cycle []string

	var visit func(n string) bool
	visit = func(n string) bool {
		s := st[n]
		if s == visiting {
			cycle = append(cycle, n)
			return true
		}
		if s == visited {
			return false
		}
		st[n] = visiting
		if p, ok := parent[n]; ok {
			if visit(p) {
				cycle = append(cycle, n)
				return true
			}
		}
		st[n] = visited
		return false
	}

	for _, p := range projects {
		if st[p.Name] == unvisited {
			if visit(p.Name) {
				// Reverse for nicer message.
				reverseStrings(cycle)
				return fmt.Errorf("project parent cycle detected: %s", strings.Join(cycle, " -> "))
			}
		}
	}
	return nil
}

func reverseStrings(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// ConfigDirPath returns the directory for optional local config/state.
// Currently used for token discovery docs and future state files.
func ConfigDirPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "todoist"), nil
}

// SortedProjectNames returns the project names sorted for deterministic output.
func (c *TodoistConfig) SortedProjectNames() []string {
	names := make([]string, 0, len(c.Spec.Projects))
	for _, p := range c.Spec.Projects {
		names = append(names, p.Name)
	}
	sort.Strings(names)
	return names
}
