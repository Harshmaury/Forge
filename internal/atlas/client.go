// @forge-project: forge
// @forge-path: internal/atlas/client.go
// Package atlas provides an HTTP client for querying the Atlas knowledge API.
// Forge reads workspace context from Atlas to enrich command objects.
// Atlas is read-only — Forge never modifies Atlas indexes.
// ADR-003: HTTP/JSON on 127.0.0.1:8081.
package atlas

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const defaultTimeout = 10 * time.Second

// ── TYPES ─────────────────────────────────────────────────────────────────────

// ProjectDetail is the project record returned by GET /workspace/project/:id.
type ProjectDetail struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Language string `json:"language"`
	Type     string `json:"type"`
	Source   string `json:"source"`
}

// WorkspaceContext is the workspace snapshot returned by GET /workspace/context.
type WorkspaceContext struct {
	WorkspaceRoot  string           `json:"workspace_root"`
	TotalFiles     int              `json:"total_files"`
	TotalDocuments int              `json:"total_documents"`
	Languages      []string         `json:"languages"`
	Projects       []*ProjectDetail `json:"projects"`
}

// ── CLIENT ────────────────────────────────────────────────────────────────────

// Client queries the Atlas HTTP API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates an Atlas Client.
func New(atlasAddr string) *Client {
	return &Client{
		baseURL:    atlasAddr,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// Ping checks whether the Atlas service is reachable.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("atlas: ping build request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("atlas unreachable at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("atlas health: HTTP %d", resp.StatusCode)
	}
	return nil
}

// GetProject fetches project detail from the Atlas workspace index.
// Returns nil, nil if the project is not indexed.
func (c *Client) GetProject(ctx context.Context, id string) (*ProjectDetail, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/workspace/project/"+id, nil)
	if err != nil {
		return nil, fmt.Errorf("atlas: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("atlas: GET /workspace/project/%s: %w", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("atlas: GET /workspace/project/%s: HTTP %d", id, resp.StatusCode)
	}

	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Project *ProjectDetail `json:"project"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("atlas: decode project response: %w", err)
	}
	if !envelope.OK {
		return nil, fmt.Errorf("atlas: API returned ok=false for project %s", id)
	}
	return envelope.Data.Project, nil
}

// GetWorkspaceContext fetches the full workspace context snapshot.
// Used to populate command context when not supplied by the caller.
func (c *Client) GetWorkspaceContext(ctx context.Context) (*WorkspaceContext, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/workspace/context", nil)
	if err != nil {
		return nil, fmt.Errorf("atlas: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("atlas: GET /workspace/context: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("atlas: GET /workspace/context: HTTP %d", resp.StatusCode)
	}

	var envelope struct {
		OK   bool             `json:"ok"`
		Data WorkspaceContext `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("atlas: decode workspace context: %w", err)
	}
	if !envelope.OK {
		return nil, fmt.Errorf("atlas: GET /workspace/context returned ok=false")
	}
	return &envelope.Data, nil
}
