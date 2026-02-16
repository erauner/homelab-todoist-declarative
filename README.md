# homelab-todoist-declarative (htd)

Declarative (GitOps-style) Todoist configuration reconciler.

This tool is intentionally **not** for day-to-day task CRUD. It manages the *non-ad-hoc* structure of a Todoist setup (projects, labels, saved filters, and optional recurring task templates) from YAML committed to git, using reconciliation semantics (plan/apply) similar to Terraform or Kubernetes.

## Status

MVP implemented:

- Projects (name identity, color/favorite/view_style + parent relationship)
- Labels (name identity, color/favorite)
- Saved Filters (name identity, query/color/favorite/order) via `/sync` commands
- Optional recurring task templates via Unified API tasks endpoints

Sections and reminders are deferred.

## CLI

Binary name: `htd`.

```bash
# Show what would change (no mutations)
htd plan -f todoist.yaml

# Apply with confirmation
htd apply -f todoist.yaml

# Apply non-interactively
htd apply -f todoist.yaml --yes

# JSON output (plan or apply)
htd plan -f todoist.yaml --json
```

Exit codes:

- `0`: success / no changes
- `2`: plan has changes (plan mode)
- non-zero: error (including aborted apply)

### Deletions (prune)

Deletes are **disabled by default**.

To enable deletions you must provide *both*:

1) CLI flag: `--prune`
2) Config gate: `spec.prune.<kind>: true`

Supported prune gates:

- `spec.prune.projects`
- `spec.prune.labels`
- `spec.prune.filters`

## Authentication / Token Discovery

Token discovery follows this strict convention:

1) If `TODOIST_API_TOKEN` env var is set, use it.
2) Else, fall back to `~/.config/todoist/config.json` with:

```json
{ "token": "<your token>" }
```

The token is treated as a secret and is never printed.

## APIs used

- Todoist **Unified API v1** for normal objects:
  - projects
  - labels
- Todoist **/sync** endpoint for objects that require command mutations:
  - filters (saved filters)
  - project parent moves (because parent changes are exposed as a `/sync` command)

## Behavior

- **Safe by default:** `plan` never mutates; `apply` requires interactive confirmation unless `--yes`.
- **Idempotent:** repeated applies with no config changes converge to “no changes”.
- **Deterministic output:** plan operations are sorted by kind then name.
- **HTTP timeout:** 30 seconds per request.
- **429 handling:** retries with exponential backoff; respects `Retry-After` when present.
- **Logging:** quiet by default; `--verbose` logs request/response status lines (never the token).

## YAML schema (MVP)

```yaml
name: personal

prune:
  projects: false
  labels: false
  filters: false
  tasks: false

projects:
  - name: Work
    color: red
    is_favorite: true
    view_style: list

  - name: Homelab
    parent: Work
    color: blue

labels:
  - name: waiting
    color: grey
    is_favorite: true

filters:
  - name: Work Focus
    query: "(today | overdue) & #Work"
    color: red
    is_favorite: true
    order: 1

tasks:
  - key: morning_review
    type: recurring_template
    content: Morning Review
    project: Work
    labels: [waiting]
    priority: 3
    due:
      string: "every day at 8:00am"
```

Notes:

- The recommended format above is intentionally not Kubernetes-shaped.
- For backwards compatibility, `htd` also accepts the older envelope format (`apiVersion`/`kind`/`metadata`/`spec`) if you already have files in that style.

### Identity + reconciliation rules

- **Projects**
  - Identity key: `name`
  - Supports parent reference: `parent: <project name>`
  - Managed fields (when present in YAML): `color`, `is_favorite`, `view_style`
  - Parent relationship is managed (omitting `parent` means *root*)
  - Deletion requires `--prune` and `spec.prune.projects: true`

- **Labels**
  - Identity key: `name`
  - Managed fields (when present in YAML): `color`, `is_favorite`
  - Deletion requires `--prune` and `spec.prune.labels: true`

- **Filters (saved filters)**
  - Identity key: `name`
  - Managed fields: `query`, `color`, `is_favorite`, `order`
  - Implemented via `/sync` commands: `filter_add`, `filter_update`, `filter_delete`, `filter_update_orders`
  - Deletion requires `--prune` and `spec.prune.filters: true`

- **Tasks (optional managed templates)**
  - Identity key: `id` or `key` (recommended: `key`)
  - `type: recurring_template` supports codifying recurring template tasks intentionally
  - Managed fields: `content`, `description`, `project`, `labels`, `priority`, `due.string`
  - Managed-by-key tasks store an internal marker line in description: `HTD_KEY:<key>`
  - Deletion requires `--prune` and `spec.prune.tasks: true` and only applies to HTD-managed tasks

### Rename behavior (current MVP)

Because identity is name-only, a “rename” is treated as:

- create new object with the new name
- (optional) delete the old object only if prune is enabled and gated

## Filter examples

These are valid examples of Todoist filter queries you can store in `spec.filters[*].query`:

1) `(today | overdue) & #Work`
2) `7 days & @waiting`
3) `!assigned & shared`
4) `created before: -365 days`
5) `p1 & overdue, p4 & today`

Note on commas: Todoist’s filter language supports comma-separated multiple queries to show multiple task lists in one filter view.

## Development

Requires Go 1.22+.

```bash
go test ./...

go build ./cmd/htd
```
