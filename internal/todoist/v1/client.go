package v1

import (
	"context"
	"fmt"
	"net/url"

	todoisthttp "github.com/erauner/homelab-todoist-declarative/internal/todoist/http"
)

type Client struct {
	http *todoisthttp.Client
}

func New(http *todoisthttp.Client) *Client { return &Client{http: http} }

type Project struct {
	ID string `json:"id"`

	Name       string  `json:"name"`
	Color      string  `json:"color"`
	IsFavorite bool    `json:"is_favorite"`
	ViewStyle  string  `json:"view_style"`
	ParentID   *string `json:"parent_id"`

	InboxProject bool `json:"inbox_project"`
}

type Label struct {
	ID string `json:"id"`

	Name       string `json:"name"`
	Color      string `json:"color"`
	IsFavorite bool   `json:"is_favorite"`
}

type Due struct {
	String      string `json:"string"`
	IsRecurring bool   `json:"is_recurring"`
}

type Task struct {
	ID string `json:"id"`

	Content     string   `json:"content"`
	Description string   `json:"description"`
	ProjectID   string   `json:"project_id"`
	Labels      []string `json:"labels"`
	Priority    int      `json:"priority"`
	Due         *Due     `json:"due"`
}

type listResponse[T any] struct {
	Results    []T     `json:"results"`
	NextCursor *string `json:"next_cursor"`
}

func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	var all []Project
	var cursor *string
	for {
		path := "/api/v1/projects"
		if cursor != nil && *cursor != "" {
			q := url.Values{}
			q.Set("cursor", *cursor)
			path = path + "?" + q.Encode()
		}
		var resp listResponse[Project]
		if err := c.http.DoJSON(ctx, "GET", path, nil, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Results...)
		if resp.NextCursor == nil || *resp.NextCursor == "" {
			break
		}
		cursor = resp.NextCursor
	}
	return all, nil
}

type CreateProjectRequest struct {
	Name       string  `json:"name"`
	ParentID   *string `json:"parent_id,omitempty"`
	Color      *string `json:"color,omitempty"`
	IsFavorite *bool   `json:"is_favorite,omitempty"`
	ViewStyle  *string `json:"view_style,omitempty"`
}

func (c *Client) CreateProject(ctx context.Context, req CreateProjectRequest) (*Project, error) {
	var resp Project
	if err := c.http.DoJSON(ctx, "POST", "/api/v1/projects", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type UpdateProjectRequest struct {
	Name       *string `json:"name,omitempty"`
	Color      *string `json:"color,omitempty"`
	IsFavorite *bool   `json:"is_favorite,omitempty"`
	ViewStyle  *string `json:"view_style,omitempty"`
}

func (c *Client) UpdateProject(ctx context.Context, projectID string, req UpdateProjectRequest) (*Project, error) {
	var resp Project
	path := fmt.Sprintf("/api/v1/projects/%s", url.PathEscape(projectID))
	if err := c.http.DoJSON(ctx, "POST", path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) DeleteProject(ctx context.Context, projectID string) error {
	path := fmt.Sprintf("/api/v1/projects/%s", url.PathEscape(projectID))
	return c.http.DoJSON(ctx, "DELETE", path, nil, nil)
}

func (c *Client) ListLabels(ctx context.Context) ([]Label, error) {
	var all []Label
	var cursor *string
	for {
		path := "/api/v1/labels"
		if cursor != nil && *cursor != "" {
			q := url.Values{}
			q.Set("cursor", *cursor)
			path = path + "?" + q.Encode()
		}
		var resp listResponse[Label]
		if err := c.http.DoJSON(ctx, "GET", path, nil, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Results...)
		if resp.NextCursor == nil || *resp.NextCursor == "" {
			break
		}
		cursor = resp.NextCursor
	}
	return all, nil
}

type CreateLabelRequest struct {
	Name       string  `json:"name"`
	Color      *string `json:"color,omitempty"`
	IsFavorite *bool   `json:"is_favorite,omitempty"`
}

func (c *Client) CreateLabel(ctx context.Context, req CreateLabelRequest) (*Label, error) {
	var resp Label
	if err := c.http.DoJSON(ctx, "POST", "/api/v1/labels", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type UpdateLabelRequest struct {
	Name       *string `json:"name,omitempty"`
	Color      *string `json:"color,omitempty"`
	IsFavorite *bool   `json:"is_favorite,omitempty"`
}

func (c *Client) UpdateLabel(ctx context.Context, labelID string, req UpdateLabelRequest) (*Label, error) {
	var resp Label
	path := fmt.Sprintf("/api/v1/labels/%s", url.PathEscape(labelID))
	if err := c.http.DoJSON(ctx, "POST", path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) DeleteLabel(ctx context.Context, labelID string) error {
	path := fmt.Sprintf("/api/v1/labels/%s", url.PathEscape(labelID))
	return c.http.DoJSON(ctx, "DELETE", path, nil, nil)
}

func (c *Client) ListTasks(ctx context.Context) ([]Task, error) {
	var all []Task
	var cursor *string
	for {
		path := "/api/v1/tasks"
		if cursor != nil && *cursor != "" {
			q := url.Values{}
			q.Set("cursor", *cursor)
			path = path + "?" + q.Encode()
		}
		var resp listResponse[Task]
		if err := c.http.DoJSON(ctx, "GET", path, nil, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Results...)
		if resp.NextCursor == nil || *resp.NextCursor == "" {
			break
		}
		cursor = resp.NextCursor
	}
	return all, nil
}

type CreateTaskRequest struct {
	Content     string   `json:"content"`
	Description *string  `json:"description,omitempty"`
	ProjectID   *string  `json:"project_id,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	Priority    *int     `json:"priority,omitempty"`
	DueString   *string  `json:"due_string,omitempty"`
}

func (c *Client) CreateTask(ctx context.Context, req CreateTaskRequest) (*Task, error) {
	var resp Task
	if err := c.http.DoJSON(ctx, "POST", "/api/v1/tasks", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type UpdateTaskRequest struct {
	Content     *string   `json:"content,omitempty"`
	Description *string   `json:"description,omitempty"`
	ProjectID   *string   `json:"project_id,omitempty"`
	Labels      *[]string `json:"labels,omitempty"`
	Priority    *int      `json:"priority,omitempty"`
	DueString   *string   `json:"due_string,omitempty"`
}

func (c *Client) UpdateTask(ctx context.Context, taskID string, req UpdateTaskRequest) (*Task, error) {
	var resp Task
	path := fmt.Sprintf("/api/v1/tasks/%s", url.PathEscape(taskID))
	if err := c.http.DoJSON(ctx, "POST", path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) DeleteTask(ctx context.Context, taskID string) error {
	path := fmt.Sprintf("/api/v1/tasks/%s", url.PathEscape(taskID))
	return c.http.DoJSON(ctx, "DELETE", path, nil, nil)
}
