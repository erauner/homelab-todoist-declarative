package sync

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	todoisthttp "github.com/erauner/homelab-todoist-declarative/internal/todoist/http"
)

func TestRunCommands_FormEncoding(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/sync" {
			t.Fatalf("expected path /api/v1/sync, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer testtoken" {
			t.Fatalf("expected Authorization Bearer testtoken, got %q", got)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
			t.Fatalf("expected form content-type, got %q", r.Header.Get("Content-Type"))
		}

		b, _ := io.ReadAll(r.Body)
		vals, err := url.ParseQuery(string(b))
		if err != nil {
			t.Fatalf("parse form: %v", err)
		}
		cmdsJSON := vals.Get("commands")
		if cmdsJSON == "" {
			t.Fatalf("expected commands field")
		}
		var cmds []map[string]any
		if err := json.Unmarshal([]byte(cmdsJSON), &cmds); err != nil {
			t.Fatalf("decode commands json: %v", err)
		}
		if len(cmds) != 3 {
			t.Fatalf("expected 3 commands, got %d", len(cmds))
		}
		if cmds[0]["type"] != "filter_add" {
			t.Fatalf("expected filter_add, got %#v", cmds[0]["type"])
		}
		if cmds[1]["type"] != "filter_update" {
			t.Fatalf("expected filter_update, got %#v", cmds[1]["type"])
		}
		if cmds[2]["type"] != "filter_delete" {
			t.Fatalf("expected filter_delete, got %#v", cmds[2]["type"])
		}

		// Generate a sync_status response referencing the received UUIDs.
		status := map[string]any{}
		for _, c := range cmds {
			status[c["uuid"].(string)] = "ok"
		}
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{"sync_status": status}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	h := todoisthttp.New("testtoken", todoisthttp.WithBaseURL(server.URL))
	c := New(h)

	cmds := []Command{
		NewTempIDCommand("filter_add", "TEMP", map[string]any{"name": "Important", "query": "priority 1"}),
		NewCommand("filter_update", map[string]any{"id": "123", "query": "priority 4"}),
		NewCommand("filter_delete", map[string]any{"id": "999"}),
	}

	ctx := context.Background()
	resp, err := c.RunCommands(ctx, cmds)
	if err != nil {
		t.Fatalf("RunCommands error: %v", err)
	}
	if err := RequireAllOK(resp, cmds); err != nil {
		t.Fatalf("RequireAllOK error: %v", err)
	}
}
