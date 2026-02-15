package v1

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	todoisthttp "github.com/erauner/homelab-todoist-declarative/internal/todoist/http"
)

func TestCreateProject_RequestShape(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/projects" {
			t.Fatalf("expected path /api/v1/projects, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer testtoken" {
			t.Fatalf("expected Authorization Bearer testtoken, got %q", got)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected Content-Type application/json, got %q", ct)
		}
		b, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(b, &payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if payload["name"] != "Work" {
			t.Fatalf("expected name Work, got %#v", payload["name"])
		}
		if payload["parent_id"] != "PARENT" {
			t.Fatalf("expected parent_id PARENT, got %#v", payload["parent_id"])
		}
		if payload["color"] != "red" {
			t.Fatalf("expected color red, got %#v", payload["color"])
		}

		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"123","name":"Work","color":"red","is_favorite":false,"view_style":"list","parent_id":"PARENT","inbox_project":false}`)
	}))
	defer server.Close()

	h := todoisthttp.New("testtoken", todoisthttp.WithBaseURL(server.URL))
	c := New(h)

	parent := "PARENT"
	color := "red"
	ctx := context.Background()
	p, err := c.CreateProject(ctx, CreateProjectRequest{Name: "Work", ParentID: &parent, Color: &color})
	if err != nil {
		t.Fatalf("CreateProject error: %v", err)
	}
	if p.ID != "123" {
		t.Fatalf("expected id 123, got %s", p.ID)
	}
}

func TestUpdateLabel_RequestShape(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/labels/42" {
			t.Fatalf("expected path /api/v1/labels/42, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer testtoken" {
			t.Fatalf("expected Authorization Bearer testtoken, got %q", got)
		}
		b, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(b, &payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if payload["color"] != "blue" {
			t.Fatalf("expected color blue, got %#v", payload["color"])
		}
		if payload["is_favorite"] != true {
			t.Fatalf("expected is_favorite true, got %#v", payload["is_favorite"])
		}

		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"42","name":"waiting","color":"blue","is_favorite":true}`)
	}))
	defer server.Close()

	h := todoisthttp.New("testtoken", todoisthttp.WithBaseURL(server.URL))
	c := New(h)

	color := "blue"
	fav := true
	ctx := context.Background()
	_, err := c.UpdateLabel(ctx, "42", UpdateLabelRequest{Color: &color, IsFavorite: &fav})
	if err != nil {
		t.Fatalf("UpdateLabel error: %v", err)
	}
}
