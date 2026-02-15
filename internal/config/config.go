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
	Prune    PruneSpec     `yaml:"prune"`
}

type PruneSpec struct {
	Projects bool `yaml:"projects"`
	Labels   bool `yaml:"labels"`
	Filters  bool `yaml:"filters"`
}

type ProjectSpec struct {
	Name       string  `yaml:"name"`
	Parent     *string `yaml:"parent,omitempty"`
	Color      *string `yaml:"color,omitempty"`
	IsFavorite *bool   `yaml:"is_favorite,omitempty"`
	ViewStyle  *string `yaml:"view_style,omitempty"`
}

type LabelSpec struct {
	Name       string  `yaml:"name"`
	Color      *string `yaml:"color,omitempty"`
	IsFavorite *bool   `yaml:"is_favorite,omitempty"`
}

type FilterSpec struct {
	Name       string  `yaml:"name"`
	Query      string  `yaml:"query"`
	Color      *string `yaml:"color,omitempty"`
	IsFavorite *bool   `yaml:"is_favorite,omitempty"`
	Order      *int    `yaml:"order,omitempty"`
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
		if c.Spec.Labels[i].Color != nil {
			col := strings.TrimSpace(*c.Spec.Labels[i].Color)
			c.Spec.Labels[i].Color = &col
		}
	}
	for i := range c.Spec.Filters {
		c.Spec.Filters[i].Name = strings.TrimSpace(c.Spec.Filters[i].Name)
		c.Spec.Filters[i].Query = strings.TrimSpace(c.Spec.Filters[i].Query)
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
	}

	// Filters: names unique, query required, order positive.
	filterNames := make(map[string]struct{}, len(c.Spec.Filters))
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
		if f.Query == "" {
			errs = append(errs, fmt.Errorf("spec.filters[%d] (%q).query is required", i, f.Name))
		}
		if f.Order == nil || *f.Order <= 0 {
			errs = append(errs, fmt.Errorf("spec.filters[%d] (%q).order must be >= 1", i, f.Name))
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
