package reconcile

import (
	"fmt"
	"sort"

	"github.com/erauner/homelab-todoist-declarative/internal/config"
)

type Options struct {
	Prune bool
}

func BuildPlan(cfg *config.TodoistConfig, snap *Snapshot, opts Options) (*Plan, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if snap == nil {
		return nil, fmt.Errorf("snapshot is nil")
	}

	plan := &Plan{}

	pruneProjects := opts.Prune && cfg.Spec.Prune.Projects
	pruneLabels := opts.Prune && cfg.Spec.Prune.Labels
	pruneFilters := opts.Prune && cfg.Spec.Prune.Filters

	// Projects
	desiredProjects := map[string]config.ProjectSpec{}
	for _, p := range cfg.Spec.Projects {
		desiredProjects[p.Name] = p
		remote, exists := snap.ProjectByName(p.Name)
		if !exists {
			plan.Operations = append(plan.Operations, Operation{
				Kind:   KindProject,
				Action: ActionCreate,
				Name:   p.Name,
				ProjectPayload: &ProjectPayload{
					DesiredName: p.Name,
					ParentName:  p.Parent,
					Color:       p.Color,
					IsFavorite:  p.IsFavorite,
					ViewStyle:   p.ViewStyle,
				},
			})
			plan.Summary.Create++
			continue
		}

		// Update managed fields via Unified API v1.
		var changes []Change
		if p.Color != nil && remote.Color != *p.Color {
			changes = append(changes, Change{Field: "color", From: remote.Color, To: *p.Color})
		}
		if p.IsFavorite != nil && remote.IsFavorite != *p.IsFavorite {
			changes = append(changes, Change{Field: "is_favorite", From: fmt.Sprintf("%t", remote.IsFavorite), To: fmt.Sprintf("%t", *p.IsFavorite)})
		}
		if p.ViewStyle != nil && remote.ViewStyle != *p.ViewStyle {
			changes = append(changes, Change{Field: "view_style", From: remote.ViewStyle, To: *p.ViewStyle})
		}
		if len(changes) > 0 {
			plan.Operations = append(plan.Operations, Operation{
				Kind:   KindProject,
				Action: ActionUpdate,
				Name:   p.Name,
				ID:     remote.ID,
				Changes: changes,
				ProjectPayload: &ProjectPayload{
					DesiredName: p.Name,
					Color:       p.Color,
					IsFavorite:  p.IsFavorite,
					ViewStyle:   p.ViewStyle,
				},
			})
			plan.Summary.Update++
		}

		// Parent (project_move) via /sync.
		desiredParent := ""
		if p.Parent != nil {
			desiredParent = *p.Parent
		}
		remoteParent := ""
		if remote.ParentID != nil {
			if name, ok := snap.ProjectNameByID(*remote.ParentID); ok {
				remoteParent = name
			} else {
				remoteParent = *remote.ParentID
			}
		}
		if desiredParent != remoteParent {
			plan.Operations = append(plan.Operations, Operation{
				Kind:   KindProject,
				Action: ActionMove,
				Name:   p.Name,
				ID:     remote.ID,
				Changes: []Change{{Field: "parent", From: remoteParent, To: desiredParent}},
				ProjectPayload: &ProjectPayload{
					DesiredName: p.Name,
					ParentName:  p.Parent,
				},
			})
			plan.Summary.Move++
		}
	}

	// Projects: deletes (prune gated)
	if opts.Prune && !cfg.Spec.Prune.Projects {
		plan.Notes = append(plan.Notes, "--prune set but spec.prune.projects=false; project deletions are disabled")
	}
	if pruneProjects {
		for _, rp := range snap.Projects {
			if _, ok := desiredProjects[rp.Name]; ok {
				continue
			}
			if rp.InboxProject {
				plan.Notes = append(plan.Notes, fmt.Sprintf("refusing to delete inbox project %q", rp.Name))
				continue
			}
			plan.Operations = append(plan.Operations, Operation{
				Kind:   KindProject,
				Action: ActionDelete,
				Name:   rp.Name,
				ID:     rp.ID,
			})
			plan.Summary.Delete++
		}
	} else {
		// Informational note: unmanaged remote projects.
		var extras int
		for _, rp := range snap.Projects {
			if _, ok := desiredProjects[rp.Name]; !ok {
				extras++
			}
		}
		if extras > 0 {
			plan.Notes = append(plan.Notes, fmt.Sprintf("%d remote projects are not in config (prune disabled)", extras))
		}
	}

	// Labels
	desiredLabels := map[string]config.LabelSpec{}
	for _, l := range cfg.Spec.Labels {
		desiredLabels[l.Name] = l
		remote, exists := snap.LabelByName(l.Name)
		if !exists {
			plan.Operations = append(plan.Operations, Operation{
				Kind:   KindLabel,
				Action: ActionCreate,
				Name:   l.Name,
				LabelPayload: &LabelPayload{
					DesiredName: l.Name,
					Color:       l.Color,
					IsFavorite:  l.IsFavorite,
				},
			})
			plan.Summary.Create++
			continue
		}
		var changes []Change
		if l.Color != nil && remote.Color != *l.Color {
			changes = append(changes, Change{Field: "color", From: remote.Color, To: *l.Color})
		}
		if l.IsFavorite != nil && remote.IsFavorite != *l.IsFavorite {
			changes = append(changes, Change{Field: "is_favorite", From: fmt.Sprintf("%t", remote.IsFavorite), To: fmt.Sprintf("%t", *l.IsFavorite)})
		}
		if len(changes) > 0 {
			plan.Operations = append(plan.Operations, Operation{
				Kind:   KindLabel,
				Action: ActionUpdate,
				Name:   l.Name,
				ID:     remote.ID,
				Changes: changes,
				LabelPayload: &LabelPayload{
					DesiredName: l.Name,
					Color:       l.Color,
					IsFavorite:  l.IsFavorite,
				},
			})
			plan.Summary.Update++
		}
	}
	if opts.Prune && !cfg.Spec.Prune.Labels {
		plan.Notes = append(plan.Notes, "--prune set but spec.prune.labels=false; label deletions are disabled")
	}
	if pruneLabels {
		for _, rl := range snap.Labels {
			if _, ok := desiredLabels[rl.Name]; ok {
				continue
			}
			plan.Operations = append(plan.Operations, Operation{
				Kind:   KindLabel,
				Action: ActionDelete,
				Name:   rl.Name,
				ID:     rl.ID,
			})
			plan.Summary.Delete++
		}
	} else {
		var extras int
		for _, rl := range snap.Labels {
			if _, ok := desiredLabels[rl.Name]; !ok {
				extras++
			}
		}
		if extras > 0 {
			plan.Notes = append(plan.Notes, fmt.Sprintf("%d remote labels are not in config (prune disabled)", extras))
		}
	}

	// Filters
	desiredFilters := map[string]config.FilterSpec{}
	for _, f := range cfg.Spec.Filters {
		desiredFilters[f.Name] = f
		remote, exists := snap.FilterByName(f.Name)
		ord := 0
		if f.Order != nil {
			ord = *f.Order
		}
		if !exists {
			plan.Operations = append(plan.Operations, Operation{
				Kind:   KindFilter,
				Action: ActionCreate,
				Name:   f.Name,
				FilterPayload: &FilterPayload{
					DesiredName: f.Name,
					Query:       f.Query,
					Color:       f.Color,
					IsFavorite:  f.IsFavorite,
					Order:       ord,
				},
			})
			plan.Summary.Create++
			continue
		}
		var changes []Change
		if remote.Query != f.Query {
			changes = append(changes, Change{Field: "query", From: remote.Query, To: f.Query})
		}
		if f.Color != nil && remote.Color != *f.Color {
			changes = append(changes, Change{Field: "color", From: remote.Color, To: *f.Color})
		}
		if f.IsFavorite != nil && remote.IsFavorite != *f.IsFavorite {
			changes = append(changes, Change{Field: "is_favorite", From: fmt.Sprintf("%t", remote.IsFavorite), To: fmt.Sprintf("%t", *f.IsFavorite)})
		}
		if ord != 0 && remote.ItemOrder != ord {
			changes = append(changes, Change{Field: "order", From: fmt.Sprintf("%d", remote.ItemOrder), To: fmt.Sprintf("%d", ord)})
		}
		if len(changes) > 0 {
			plan.Operations = append(plan.Operations, Operation{
				Kind:   KindFilter,
				Action: ActionUpdate,
				Name:   f.Name,
				ID:     remote.ID,
				Changes: changes,
				FilterPayload: &FilterPayload{
					DesiredName: f.Name,
					Query:       f.Query,
					Color:       f.Color,
					IsFavorite:  f.IsFavorite,
					Order:       ord,
					RemoteID:    remote.ID,
				},
			})
			plan.Summary.Update++
		}
	}
	if opts.Prune && !cfg.Spec.Prune.Filters {
		plan.Notes = append(plan.Notes, "--prune set but spec.prune.filters=false; filter deletions are disabled")
	}
	if pruneFilters {
		for _, rf := range snap.Filters {
			if _, ok := desiredFilters[rf.Name]; ok {
				continue
			}
			plan.Operations = append(plan.Operations, Operation{
				Kind:   KindFilter,
				Action: ActionDelete,
				Name:   rf.Name,
				ID:     rf.ID,
				FilterPayload: &FilterPayload{RemoteID: rf.ID, DesiredName: rf.Name},
			})
			plan.Summary.Delete++
		}
	} else {
		var extras int
		for _, rf := range snap.Filters {
			if _, ok := desiredFilters[rf.Name]; !ok {
				extras++
			}
		}
		if extras > 0 {
			plan.Notes = append(plan.Notes, fmt.Sprintf("%d remote filters are not in config (prune disabled)", extras))
		}
	}

	// Deterministic ordering: sort by kind then name, then action.
	sort.Slice(plan.Operations, func(i, j int) bool {
		a, b := plan.Operations[i], plan.Operations[j]
		if a.Kind != b.Kind {
			return kindOrder(a.Kind) < kindOrder(b.Kind)
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Action < b.Action
	})

	return plan, nil
}

func kindOrder(k Kind) int {
	switch k {
	case KindProject:
		return 0
	case KindLabel:
		return 1
	case KindFilter:
		return 2
	default:
		return 99
	}
}
