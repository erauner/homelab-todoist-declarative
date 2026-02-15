package reconcile

import (
	"context"
	"fmt"
	"sort"

	"github.com/google/uuid"

	"github.com/erauner/homelab-todoist-declarative/internal/config"
	todoistsync "github.com/erauner/homelab-todoist-declarative/internal/todoist/sync"
	"github.com/erauner/homelab-todoist-declarative/internal/todoist/v1"
)

type Clients struct {
	V1   *v1.Client
	Sync *todoistsync.Client
}

func Apply(ctx context.Context, cfg *config.TodoistConfig, snap *Snapshot, plan *Plan, clients Clients, opts Options) (*ApplyResult, error) {
	if cfg == nil || snap == nil || plan == nil {
		return nil, fmt.Errorf("cfg/snapshot/plan must be non-nil")
	}
	if clients.V1 == nil || clients.Sync == nil {
		return nil, fmt.Errorf("todoist clients must be non-nil")
	}

	res := &ApplyResult{Summary: plan.Summary}

	// Precompute project name -> id map (updated as we create).
	projectNameToID := map[string]string{}
	for _, p := range snap.Projects {
		projectNameToID[p.Name] = p.ID
	}

	// --- Projects: Create (topological by parent)
	projectCreates := filterOps(plan.Operations, KindProject, ActionCreate)
	sortedCreates, err := topoSortProjectCreates(projectCreates)
	if err != nil {
		return nil, err
	}
	for _, op := range sortedCreates {
		payload := op.ProjectPayload
		if payload == nil {
			return nil, fmt.Errorf("project create op missing payload for %q", op.Name)
		}
		var parentID *string
		if payload.ParentName != nil {
			pid, ok := projectNameToID[*payload.ParentName]
			if !ok {
				return nil, fmt.Errorf("project %q parent %q not found (create ordering bug)", op.Name, *payload.ParentName)
			}
			parentID = &pid
		}
		created, err := clients.V1.CreateProject(ctx, v1.CreateProjectRequest{
			Name:       payload.DesiredName,
			ParentID:   parentID,
			Color:      payload.Color,
			IsFavorite: payload.IsFavorite,
			ViewStyle:  payload.ViewStyle,
		})
		if err != nil {
			return nil, fmt.Errorf("create project %q: %w", op.Name, err)
		}
		projectNameToID[created.Name] = created.ID
		res.Applied = append(res.Applied, OperationResult{Kind: KindProject, Action: ActionCreate, Name: op.Name, ID: created.ID, Status: "ok"})
	}

	// --- Projects: Update (Unified API)
	projectUpdates := filterOps(plan.Operations, KindProject, ActionUpdate)
	sort.Slice(projectUpdates, func(i, j int) bool { return projectUpdates[i].Name < projectUpdates[j].Name })
	for _, op := range projectUpdates {
		payload := op.ProjectPayload
		if payload == nil {
			return nil, fmt.Errorf("project update op missing payload for %q", op.Name)
		}
		req := v1.UpdateProjectRequest{}
		for _, ch := range op.Changes {
			switch ch.Field {
			case "name":
				n := payload.DesiredName
				req.Name = &n
			case "color":
				req.Color = payload.Color
			case "is_favorite":
				req.IsFavorite = payload.IsFavorite
			case "view_style":
				req.ViewStyle = payload.ViewStyle
			}
		}
		_, err := clients.V1.UpdateProject(ctx, op.ID, req)
		if err != nil {
			return nil, fmt.Errorf("update project %q: %w", op.Name, err)
		}
		res.Applied = append(res.Applied, OperationResult{Kind: KindProject, Action: ActionUpdate, Name: op.Name, ID: op.ID, Status: "ok"})
	}

	// --- Projects: Move parent (sync)
	projectMoves := filterOps(plan.Operations, KindProject, ActionMove)
	sort.Slice(projectMoves, func(i, j int) bool { return projectMoves[i].Name < projectMoves[j].Name })
	if len(projectMoves) > 0 {
		var cmds []todoistsync.Command
		for _, op := range projectMoves {
			payload := op.ProjectPayload
			if payload == nil {
				return nil, fmt.Errorf("project move op missing payload for %q", op.Name)
			}
			args := map[string]any{"id": op.ID}
			if payload.ParentName == nil {
				args["parent_id"] = nil
			} else {
				pid, ok := projectNameToID[*payload.ParentName]
				if !ok {
					return nil, fmt.Errorf("move project %q: parent %q id not found", op.Name, *payload.ParentName)
				}
				args["parent_id"] = pid
			}
			cmds = append(cmds, todoistsync.NewCommand("project_move", args))
		}
		resp, err := clients.Sync.RunCommands(ctx, cmds)
		if err != nil {
			return nil, fmt.Errorf("sync project_move: %w", err)
		}
		if err := todoistsync.RequireAllOK(resp, cmds); err != nil {
			return nil, fmt.Errorf("sync project_move statuses: %w", err)
		}
		for _, op := range projectMoves {
			res.Applied = append(res.Applied, OperationResult{Kind: KindProject, Action: ActionMove, Name: op.Name, ID: op.ID, Status: "ok"})
		}
	}

	// --- Labels
	labelCreates := filterOps(plan.Operations, KindLabel, ActionCreate)
	sort.Slice(labelCreates, func(i, j int) bool { return labelCreates[i].Name < labelCreates[j].Name })
	for _, op := range labelCreates {
		payload := op.LabelPayload
		if payload == nil {
			return nil, fmt.Errorf("label create op missing payload for %q", op.Name)
		}
		created, err := clients.V1.CreateLabel(ctx, v1.CreateLabelRequest{
			Name:       payload.DesiredName,
			Color:      payload.Color,
			IsFavorite: payload.IsFavorite,
		})
		if err != nil {
			return nil, fmt.Errorf("create label %q: %w", op.Name, err)
		}
		res.Applied = append(res.Applied, OperationResult{Kind: KindLabel, Action: ActionCreate, Name: op.Name, ID: created.ID, Status: "ok"})
	}

	labelUpdates := filterOps(plan.Operations, KindLabel, ActionUpdate)
	sort.Slice(labelUpdates, func(i, j int) bool { return labelUpdates[i].Name < labelUpdates[j].Name })
	for _, op := range labelUpdates {
		payload := op.LabelPayload
		if payload == nil {
			return nil, fmt.Errorf("label update op missing payload for %q", op.Name)
		}
		req := v1.UpdateLabelRequest{}
		for _, ch := range op.Changes {
			switch ch.Field {
			case "name":
				n := payload.DesiredName
				req.Name = &n
			case "color":
				req.Color = payload.Color
			case "is_favorite":
				req.IsFavorite = payload.IsFavorite
			}
		}
		_, err := clients.V1.UpdateLabel(ctx, op.ID, req)
		if err != nil {
			return nil, fmt.Errorf("update label %q: %w", op.Name, err)
		}
		res.Applied = append(res.Applied, OperationResult{Kind: KindLabel, Action: ActionUpdate, Name: op.Name, ID: op.ID, Status: "ok"})
	}

	// --- Filters (sync commands)
	filterNameToID := map[string]string{}
	for _, f := range snap.Filters {
		filterNameToID[f.Name] = f.ID
	}

	filterCreates := filterOps(plan.Operations, KindFilter, ActionCreate)
	filterUpdates := filterOps(plan.Operations, KindFilter, ActionUpdate)
	filterDeletes := filterOps(plan.Operations, KindFilter, ActionDelete)

	sort.Slice(filterCreates, func(i, j int) bool { return filterCreates[i].Name < filterCreates[j].Name })
	sort.Slice(filterUpdates, func(i, j int) bool { return filterUpdates[i].Name < filterUpdates[j].Name })
	sort.Slice(filterDeletes, func(i, j int) bool { return filterDeletes[i].Name < filterDeletes[j].Name })

	var filterCmds []todoistsync.Command
	tempIDToName := map[string]string{}

	for _, op := range filterCreates {
		payload := op.FilterPayload
		if payload == nil {
			return nil, fmt.Errorf("filter create op missing payload for %q", op.Name)
		}
		tempID := uuid.NewString()
		tempIDToName[tempID] = op.Name
		args := map[string]any{"name": payload.DesiredName, "query": payload.Query}
		if payload.Color != nil {
			args["color"] = *payload.Color
		}
		if payload.IsFavorite != nil {
			args["is_favorite"] = *payload.IsFavorite
		}
		if payload.Order > 0 {
			args["item_order"] = payload.Order
		}
		filterCmds = append(filterCmds, todoistsync.NewTempIDCommand("filter_add", tempID, args))
	}
	for _, op := range filterUpdates {
		payload := op.FilterPayload
		if payload == nil {
			return nil, fmt.Errorf("filter update op missing payload for %q", op.Name)
		}
		args := map[string]any{"id": payload.RemoteID}
		for _, ch := range op.Changes {
			switch ch.Field {
			case "name":
				args["name"] = payload.DesiredName
			case "query":
				args["query"] = payload.Query
			case "color":
				if payload.Color != nil {
					args["color"] = *payload.Color
				}
			case "is_favorite":
				if payload.IsFavorite != nil {
					args["is_favorite"] = *payload.IsFavorite
				}
			case "order":
				args["item_order"] = payload.Order
			}
		}
		filterCmds = append(filterCmds, todoistsync.NewCommand("filter_update", args))
	}
	for _, op := range filterDeletes {
		payload := op.FilterPayload
		if payload == nil {
			return nil, fmt.Errorf("filter delete op missing payload for %q", op.Name)
		}
		args := map[string]any{"id": payload.RemoteID}
		filterCmds = append(filterCmds, todoistsync.NewCommand("filter_delete", args))
	}

	if len(filterCmds) > 0 {
		resp, err := clients.Sync.RunCommands(ctx, filterCmds)
		if err != nil {
			return nil, fmt.Errorf("sync filter commands: %w", err)
		}
		if err := todoistsync.RequireAllOK(resp, filterCmds); err != nil {
			return nil, fmt.Errorf("sync filter statuses: %w", err)
		}

		// Map newly created filters.
		for tempID, newID := range resp.TempIDMapping {
			name, ok := tempIDToName[tempID]
			if !ok {
				continue
			}
			filterNameToID[name] = newID
		}

		for _, op := range filterCreates {
			res.Applied = append(res.Applied, OperationResult{Kind: KindFilter, Action: ActionCreate, Name: op.Name, Status: "ok"})
		}
		for _, op := range filterUpdates {
			res.Applied = append(res.Applied, OperationResult{Kind: KindFilter, Action: ActionUpdate, Name: op.Name, ID: op.ID, Status: "ok"})
		}
		for _, op := range filterDeletes {
			res.Applied = append(res.Applied, OperationResult{Kind: KindFilter, Action: ActionDelete, Name: op.Name, ID: op.ID, Status: "ok"})
		}
	}

	// Apply filter order as a bulk command for determinism when there were create/update changes.
	needFilterOrderUpdate := false
	for _, op := range append(filterCreates, filterUpdates...) {
		if op.Action == ActionCreate {
			needFilterOrderUpdate = true
			break
		}
		for _, ch := range op.Changes {
			if ch.Field == "order" {
				needFilterOrderUpdate = true
				break
			}
		}
		if needFilterOrderUpdate {
			break
		}
	}
	if needFilterOrderUpdate && len(cfg.Spec.Filters) > 0 {
		idOrder := map[string]int{}
		for _, f := range cfg.Spec.Filters {
			id := ""
			if f.ID != nil {
				id = *f.ID
			} else {
				var ok bool
				id, ok = filterNameToID[f.Name]
				if !ok {
					return nil, fmt.Errorf("filter %q id missing after create/update", f.Name)
				}
			}
			ord := 0
			if f.Order != nil {
				ord = *f.Order
			}
			idOrder[id] = ord
		}
		cmds := []todoistsync.Command{
			todoistsync.NewCommand("filter_update_orders", map[string]any{"id_order_mapping": idOrder}),
		}
		resp, err := clients.Sync.RunCommands(ctx, cmds)
		if err != nil {
			return nil, fmt.Errorf("sync filter_update_orders: %w", err)
		}
		if err := todoistsync.RequireAllOK(resp, cmds); err != nil {
			return nil, fmt.Errorf("sync filter_update_orders statuses: %w", err)
		}
		res.Applied = append(res.Applied, OperationResult{Kind: KindFilter, Action: ActionReorder, Name: "filters", Status: "ok"})
		res.Summary.Reorder++
	}

	// --- Deletes (labels, projects) last; project deletes child-first.
	// Labels
	labelDeletes := filterOps(plan.Operations, KindLabel, ActionDelete)
	sort.Slice(labelDeletes, func(i, j int) bool { return labelDeletes[i].Name < labelDeletes[j].Name })
	for _, op := range labelDeletes {
		if err := clients.V1.DeleteLabel(ctx, op.ID); err != nil {
			return nil, fmt.Errorf("delete label %q: %w", op.Name, err)
		}
		res.Applied = append(res.Applied, OperationResult{Kind: KindLabel, Action: ActionDelete, Name: op.Name, ID: op.ID, Status: "ok"})
	}

	// Projects
	projectDeletes := filterOps(plan.Operations, KindProject, ActionDelete)
	projectDeletes = sortProjectsByDepthDesc(projectDeletes, snap)
	for _, op := range projectDeletes {
		if err := clients.V1.DeleteProject(ctx, op.ID); err != nil {
			return nil, fmt.Errorf("delete project %q: %w", op.Name, err)
		}
		res.Applied = append(res.Applied, OperationResult{Kind: KindProject, Action: ActionDelete, Name: op.Name, ID: op.ID, Status: "ok"})
	}

	return res, nil
}

func filterOps(ops []Operation, kind Kind, action Action) []Operation {
	var out []Operation
	for _, op := range ops {
		if op.Kind == kind && op.Action == action {
			out = append(out, op)
		}
	}
	return out
}

func topoSortProjectCreates(creates []Operation) ([]Operation, error) {
	if len(creates) == 0 {
		return nil, nil
	}
	byName := map[string]Operation{}
	for _, op := range creates {
		byName[op.Name] = op
	}
	indegree := map[string]int{}
	children := map[string][]string{}
	for _, op := range creates {
		indegree[op.Name] = 0
	}
	for _, op := range creates {
		payload := op.ProjectPayload
		if payload == nil {
			return nil, fmt.Errorf("project create op missing payload for %q", op.Name)
		}
		if payload.ParentName == nil {
			continue
		}
		parentName := *payload.ParentName
		if parentName == "" {
			continue
		}
		if _, ok := byName[parentName]; ok {
			indegree[op.Name]++
			children[parentName] = append(children[parentName], op.Name)
		}
	}

	// Queue (sorted) for determinism.
	var q []string
	for name, deg := range indegree {
		if deg == 0 {
			q = append(q, name)
		}
	}
	sort.Strings(q)

	var out []Operation
	for len(q) > 0 {
		n := q[0]
		q = q[1:]
		out = append(out, byName[n])
		for _, child := range children[n] {
			indegree[child]--
			if indegree[child] == 0 {
				q = append(q, child)
			}
		}
		sort.Strings(q)
	}

	if len(out) != len(creates) {
		return nil, fmt.Errorf("project create ordering cycle detected")
	}
	return out, nil
}

func sortProjectsByDepthDesc(deletes []Operation, snap *Snapshot) []Operation {
	if len(deletes) == 0 {
		return deletes
	}
	// Build ID -> parent ID map from snapshot.
	parentByID := map[string]*string{}
	nameByID := map[string]string{}
	for _, p := range snap.Projects {
		pCopy := p.ParentID
		parentByID[p.ID] = pCopy
		nameByID[p.ID] = p.Name
	}
	depth := func(id string) int {
		seen := map[string]struct{}{}
		d := 0
		cur := id
		for {
			if _, ok := seen[cur]; ok {
				return d
			}
			seen[cur] = struct{}{}
			pid, ok := parentByID[cur]
			if !ok || pid == nil || *pid == "" {
				return d
			}
			d++
			cur = *pid
		}
	}
	sort.SliceStable(deletes, func(i, j int) bool {
		di := depth(deletes[i].ID)
		dj := depth(deletes[j].ID)
		if di != dj {
			return di > dj
		}
		// Tie-breaker: name.
		ni, ok1 := nameByID[deletes[i].ID]
		nj, ok2 := nameByID[deletes[j].ID]
		if ok1 && ok2 && ni != nj {
			return ni < nj
		}
		return deletes[i].Name < deletes[j].Name
	})
	return deletes
}

var _ = uuid.Nil
