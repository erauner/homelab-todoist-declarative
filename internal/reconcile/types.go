package reconcile

import "fmt"

type Kind string

const (
	KindProject Kind = "project"
	KindLabel   Kind = "label"
	KindFilter  Kind = "filter"
	KindTask    Kind = "task"
)

type Action string

const (
	ActionCreate  Action = "create"
	ActionUpdate  Action = "update"
	ActionMove    Action = "move"
	ActionDelete  Action = "delete"
	ActionReorder Action = "reorder"
)

type Change struct {
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

type Operation struct {
	Kind    Kind     `json:"kind"`
	Action  Action   `json:"action"`
	Name    string   `json:"name"`
	ID      string   `json:"id,omitempty"` // remote ID when relevant
	Changes []Change `json:"changes,omitempty"`

	// Internal-only payloads for apply.
	ProjectPayload *ProjectPayload `json:"-"`
	LabelPayload   *LabelPayload   `json:"-"`
	FilterPayload  *FilterPayload  `json:"-"`
	TaskPayload    *TaskPayload    `json:"-"`
}

func (op Operation) SortKey() string {
	return fmt.Sprintf("%s/%s/%s", op.Kind, op.Name, op.Action)
}

type Summary struct {
	Create  int `json:"create"`
	Update  int `json:"update"`
	Move    int `json:"move"`
	Delete  int `json:"delete"`
	Reorder int `json:"reorder"`
}

func (s Summary) TotalChanges() int {
	return s.Create + s.Update + s.Move + s.Delete + s.Reorder
}

type Plan struct {
	Operations []Operation `json:"operations"`
	Summary    Summary     `json:"summary"`
	Notes      []string    `json:"notes,omitempty"`
}

// ApplyResult captures outcomes per operation (best-effort; MVP only).
type ApplyResult struct {
	Applied []OperationResult `json:"applied"`
	Summary Summary           `json:"summary"`
}

type OperationResult struct {
	Kind   Kind   `json:"kind"`
	Action Action `json:"action"`
	Name   string `json:"name"`
	ID     string `json:"id,omitempty"`
	Status string `json:"status"` // "ok" or error string
}

type ProjectPayload struct {
	DesiredName string
	ParentName  *string
	Color       *string
	IsFavorite  *bool
	ViewStyle   *string
}

type LabelPayload struct {
	DesiredName string
	Color       *string
	IsFavorite  *bool
}

type FilterPayload struct {
	DesiredName string
	Query       string
	Color       *string
	IsFavorite  *bool
	Order       int
	RemoteID    string // for updates/deletes
}

type TaskPayload struct {
	Key         string
	DesiredName string
	Description *string
	ProjectName *string
	ProjectID   *string
	Labels      []string
	Priority    *int
	DueString   *string
}
