package export

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/erauner/homelab-todoist-declarative/internal/config"
	"github.com/erauner/homelab-todoist-declarative/internal/reconcile"
)

type Options struct {
	Full         bool
	IncludeInbox bool
	IncludeIDs   bool
}

// SimpleConfig is the preferred wire format for htd configs (no apiVersion/kind/metadata/spec envelope).
type SimpleConfig struct {
	Name     string               `yaml:"name" json:"name"`
	Prune    config.PruneSpec     `yaml:"prune" json:"prune"`
	Projects []config.ProjectSpec `yaml:"projects" json:"projects"`
	Labels   []config.LabelSpec   `yaml:"labels" json:"labels"`
	Filters  []config.FilterSpec  `yaml:"filters" json:"filters"`
}

func FromSnapshot(name string, snap *reconcile.Snapshot, opts Options) (*SimpleConfig, error) {
	if snap == nil {
		return nil, fmt.Errorf("snapshot is nil")
	}
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	out := &SimpleConfig{
		Name: name,
		Prune: config.PruneSpec{
			Projects: false,
			Labels:   false,
			Filters:  false,
		},
	}

	// Projects: output in parent-before-child order for readability.
	type proj struct {
		id        string
		name      string
		parent    *string
		color     string
		favorite  bool
		viewStyle string
	}
	projs := make([]proj, 0, len(snap.Projects))
	parentByName := map[string]*string{}
	for _, p := range snap.Projects {
		if p.InboxProject && !opts.IncludeInbox {
			continue
		}
		var parentName *string
		if p.ParentID != nil {
			if n, ok := snap.ProjectNameByID(*p.ParentID); ok {
				parentName = &n
			} else {
				n := *p.ParentID
				parentName = &n
			}
		}
		parentByName[p.Name] = parentName
		projs = append(projs, proj{
			id:        p.ID,
			name:      p.Name,
			parent:    parentName,
			color:     p.Color,
			favorite:  p.IsFavorite,
			viewStyle: p.ViewStyle,
		})
	}

	depthMemo := map[string]int{}
	var depth func(string) int
	depth = func(n string) int {
		if d, ok := depthMemo[n]; ok {
			return d
		}
		p := parentByName[n]
		if p == nil || *p == "" {
			depthMemo[n] = 0
			return 0
		}
		// If parent isn't in map, treat as root.
		if _, ok := parentByName[*p]; !ok {
			depthMemo[n] = 0
			return 0
		}
		d := depth(*p) + 1
		depthMemo[n] = d
		return d
	}

	sort.Slice(projs, func(i, j int) bool {
		di, dj := depth(projs[i].name), depth(projs[j].name)
		if di != dj {
			return di < dj
		}
		return projs[i].name < projs[j].name
	})

	for _, p := range projs {
		ps := config.ProjectSpec{
			Name:   p.name,
			Parent: p.parent,
		}
		if opts.IncludeIDs {
			id := p.id
			ps.ID = &id
		}
		if opts.Full {
			if p.color != "" {
				v := p.color
				ps.Color = &v
			}
			vf := p.favorite
			ps.IsFavorite = &vf
			if p.viewStyle != "" {
				vs := p.viewStyle
				ps.ViewStyle = &vs
			}
		}
		out.Projects = append(out.Projects, ps)
	}

	// Labels: name-only by default (full adds fields).
	sort.Slice(snap.Labels, func(i, j int) bool { return snap.Labels[i].Name < snap.Labels[j].Name })
	for _, l := range snap.Labels {
		ls := config.LabelSpec{Name: l.Name}
		if opts.IncludeIDs {
			id := l.ID
			ls.ID = &id
		}
		if opts.Full {
			if l.Color != "" {
				v := l.Color
				ls.Color = &v
			}
			vf := l.IsFavorite
			ls.IsFavorite = &vf
		}
		out.Labels = append(out.Labels, ls)
	}

	// Filters: query + order are always included; full adds color/favorite.
	sort.Slice(snap.Filters, func(i, j int) bool {
		if snap.Filters[i].ItemOrder != snap.Filters[j].ItemOrder {
			return snap.Filters[i].ItemOrder < snap.Filters[j].ItemOrder
		}
		return snap.Filters[i].Name < snap.Filters[j].Name
	})
	for _, f := range snap.Filters {
		ord := f.ItemOrder
		fs := config.FilterSpec{
			Name:  f.Name,
			Query: f.Query,
			Order: &ord,
		}
		if opts.IncludeIDs {
			id := f.ID
			fs.ID = &id
		}
		if opts.Full {
			if f.Color != "" {
				v := f.Color
				fs.Color = &v
			}
			vf := f.IsFavorite
			fs.IsFavorite = &vf
		}
		out.Filters = append(out.Filters, fs)
	}

	return out, nil
}

func (c *SimpleConfig) ToYAML() ([]byte, error) {
	return yaml.Marshal(c)
}
