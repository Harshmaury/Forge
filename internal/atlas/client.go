// @forge-project: forge
// @forge-path: internal/atlas/client.go
// ADR-008: serviceToken field + get() helper inject X-Service-Token on all
// outbound requests except /health.
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

// ProjectDetail is the project record returned by Atlas endpoints.
// Phase 4: includes Status, Capabilities, DependsOn from ADR-009 contract.
type ProjectDetail struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Path         string   `json:"path"`
	Language     string   `json:"language"`
	Type         string   `json:"type"`
	Source       string   `json:"source"`
	Status       string   `json:"status"`
	Capabilities []string `json:"capabilities"`
	DependsOn    []string `json:"depends_on"`
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
	baseURL      string
	httpClient   *http.Client
	serviceToken string // ADR-008
}

// New creates an Atlas Client.
func New(atlasAddr string) *Client {
	return &Client{
		baseURL:    atlasAddr,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// WithServiceToken sets the X-Service-Token header for ADR-008 inter-service auth.
func (c *Client) WithServiceToken(token string) *Client {
	c.serviceToken = token
	return c
}

// get is an authenticated GET helper — adds X-Service-Token on non-health paths.
func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if c.serviceToken != "" && path != "/health" {
		req.Header.Set("X-Service-Token", c.serviceToken) // ADR-008
	}
	return c.httpClient.Do(req)
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
	resp, err := c.get(ctx, "/workspace/project/"+id)
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

// GetVerifiedServices fetches verified projects from Atlas GET /graph/services.
// Phase 4 (ADR-010): used by preflight.Checker for pre-execution validation.
// Returns only projects with status=verified — the stable contract endpoint.
func (c *Client) GetVerifiedServices(ctx context.Context) ([]*ProjectDetail, error) {
	resp, err := c.get(ctx, "/graph/services")
	if err != nil {
		return nil, fmt.Errorf("atlas: GET /graph/services: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("atlas: GET /graph/services: HTTP %d", resp.StatusCode)
	}

	var envelope struct {
		OK   bool             `json:"ok"`
		Data []*ProjectDetail `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("atlas: decode /graph/services: %w", err)
	}
	if !envelope.OK {
		return nil, fmt.Errorf("atlas: /graph/services returned ok=false")
	}
	return envelope.Data, nil
}

// GetWorkspaceContext fetches the full workspace context snapshot.
// Used to populate command context when not supplied by the caller.
func (c *Client) GetWorkspaceContext(ctx context.Context) (*WorkspaceContext, error) {
	resp, err := c.get(ctx, "/workspace/context")
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
