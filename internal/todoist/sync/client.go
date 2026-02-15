package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/google/uuid"

	todoisthttp "github.com/erauner/homelab-todoist-declarative/internal/todoist/http"
)

type Client struct {
	http *todoisthttp.Client
}

func New(http *todoisthttp.Client) *Client { return &Client{http: http} }

// Command represents a /sync command.
// See: https://developer.todoist.com/api/v1/ ("/sync" section).
//
// Note: The /sync endpoint keeps legacy command names (e.g. filter_add, project_move).
// This tool uses /api/v1/sync.
//
// Commands are submitted as a JSON array encoded into a form field named "commands".
type Command struct {
	Type   string                 `json:"type"`
	UUID   string                 `json:"uuid"`
	TempID *string                `json:"temp_id,omitempty"`
	Args   map[string]any         `json:"args"`
}

func NewCommand(cmdType string, args map[string]any) Command {
	return Command{
		Type: cmdType,
		UUID: uuid.NewString(),
		Args: args,
	}
}

func NewTempIDCommand(cmdType string, tempID string, args map[string]any) Command {
	return Command{
		Type:   cmdType,
		UUID:   uuid.NewString(),
		TempID: &tempID,
		Args:   args,
	}
}

type Filter struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Query      string `json:"query"`
	Color      string `json:"color"`
	ItemOrder  int    `json:"item_order"`
	IsFavorite bool   `json:"is_favorite"`
	IsDeleted  bool   `json:"is_deleted"`
}

type SyncResponse struct {
	SyncStatus    map[string]any   `json:"sync_status"`
	TempIDMapping map[string]string `json:"temp_id_mapping"`

	Filters []Filter `json:"filters"`
}

// Read performs a full sync for the given resource types.
// resourceTypes examples: ["filters"].
func (c *Client) Read(ctx context.Context, resourceTypes []string) (*SyncResponse, error) {
	rt, err := json.Marshal(resourceTypes)
	if err != nil {
		return nil, fmt.Errorf("marshal resource types: %w", err)
	}
	values := url.Values{}
	values.Set("sync_token", "*")
	values.Set("resource_types", string(rt))
	var resp SyncResponse
	if err := c.http.DoForm(ctx, "/api/v1/sync", values, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// RunCommands submits /sync commands.
func (c *Client) RunCommands(ctx context.Context, commands []Command) (*SyncResponse, error) {
	b, err := json.Marshal(commands)
	if err != nil {
		return nil, fmt.Errorf("marshal commands: %w", err)
	}
	values := url.Values{}
	values.Set("commands", string(b))
	var resp SyncResponse
	if err := c.http.DoForm(ctx, "/api/v1/sync", values, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// RequireAllOK validates sync_status in a response for a set of commands.
// Any status that isn't the string "ok" is treated as an error.
func RequireAllOK(resp *SyncResponse, commands []Command) error {
	if resp == nil {
		return fmt.Errorf("sync response is nil")
	}
	if len(commands) == 0 {
		return nil
	}
	var errs []error
	for _, cmd := range commands {
		st, ok := resp.SyncStatus[cmd.UUID]
		if !ok {
			errs = append(errs, fmt.Errorf("sync_status missing uuid=%s type=%s", cmd.UUID, cmd.Type))
			continue
		}
		if s, ok := st.(string); ok {
			if s != "ok" {
				errs = append(errs, fmt.Errorf("sync_status uuid=%s type=%s: %s", cmd.UUID, cmd.Type, s))
			}
			continue
		}
		// For some commands, Todoist returns an object (e.g. LRO). Treat as error in MVP.
		errs = append(errs, fmt.Errorf("sync_status uuid=%s type=%s: unexpected non-string status", cmd.UUID, cmd.Type))
	}
	if len(errs) > 0 {
		return errorsJoin(errs)
	}
	return nil
}

func errorsJoin(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	// Go 1.22 has errors.Join; keep local to avoid importing errors in this package.
	msg := ""
	for i, e := range errs {
		if i > 0 {
			msg += "; "
		}
		msg += e.Error()
	}
	return fmt.Errorf(msg)
}
