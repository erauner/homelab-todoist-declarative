# homelab-todoist-declarative

Declarative (GitOps-style) Todoist configuration management.

## Intent

Codify the non-ad-hoc parts of a Todoist setup (projects, sections, labels, and saved filters) in a repo,
then reconcile Todoist to match the desired state with a deterministic, idempotent tool.

This is explicitly **not** for day-to-day task CRUD. It is for keeping structure and conventions stable.

## Why Not Terraform/Provider-Restapi

Todoist’s “saved filters” live in the Sync API as command-style mutations (add/update/delete/reorder) rather
than classic CRUD endpoints. A purpose-built reconciler is a better fit than forcing this into a generic
REST provider model.

## MVP Scope

- Manage **Projects** (name, parent, color, favorite, order where feasible).
- Manage **Sections** within projects.
- Manage **Labels** (name, color, favorite, order).
- Manage **Saved Filters** (name, query, color, favorite, order).
- Support `plan` (diff), `apply`, and `import` (bootstrap desired state from an existing Todoist).

Non-goals for MVP:

- Managing individual tasks.
- Complex conflict resolution across multiple human editors beyond “last apply wins”.
- Perfect drift detection for every field if the API doesn’t expose it cleanly.

## Design (High Level)

### Desired State

Single source of truth file, e.g. `todoist.yaml`:

- `projects`: stable keys, nested hierarchy
- `sections`: under projects
- `labels`
- `filters` (saved filters)

### State / ID Mapping

Todoist objects have server IDs. We will maintain a local state file (e.g. `.todoist-state.json`) that maps
stable logical keys (like `projects.work.name`) to Todoist IDs to make reconciliation reliable even if names
are edited.

### APIs Used

- REST API for projects/sections/labels where available.
- Sync API commands for saved filters and ordering.

### Reconciliation Strategy

1. Load desired config.
2. Load state (if present).
3. Fetch remote snapshot (projects/sections/labels/filters).
4. Compute diff:
   - creates, updates, deletes, reorders
5. Apply in dependency order:
   - projects (parents first)
   - sections
   - labels
   - filters
6. Write updated state.

## CLI (Planned)

```bash
todoist-decl validate -f todoist.yaml
todoist-decl plan -f todoist.yaml
todoist-decl apply -f todoist.yaml
todoist-decl import --out todoist.yaml
```

Auth:

- `TODOIST_API_TOKEN` env var, or
- `~/.config/todoist/config.json` with `{ "token": "..." }`

## Initial Implementation Plan

1. Define YAML schema + stable keying strategy.
2. Implement Todoist clients:
   - REST client (requests)
   - Sync client (command batching)
3. Implement `import` for a first usable bootstrapping flow.
4. Implement diff engine + `plan` output.
5. Implement `apply` with safe ordering + state updates.
6. Add unit tests with mocked HTTP.

## Status

Scaffold only. Implementation intentionally deferred until we settle the desired schema and reconciliation
rules.

