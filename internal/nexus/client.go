// @forge-project: forge
// @forge-path: internal/nexus/client.go
// ADR-008: serviceToken field + get() helper inject X-Service-Token on all
// outbound requests except /health.
// Package nexus provides an HTTP client for querying the Nexus API.
// Forge reads project and service state from Nexus — it never writes.
// ADR-001: Nexus is the canonical project registry.
// ADR-003: HTTP/JSON on 127.0.0.1:8080.
package nexus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	canon "github.com/Harshmaury/Canon/identity"
	"time"
)

const defaultTimeout = 10 * time.Second

// ── TYPES ─────────────────────────────────────────────────────────────────────

// Project is a project record as returned by GET /projects/:id.
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Language    string `json:"language"`
	ProjectType string `json:"project_type"`
}

// ── CLIENT ────────────────────────────────────────────────────────────────────

// Client queries the Nexus HTTP API.
type Client struct {
	baseURL      string
	httpClient   *http.Client
	serviceToken string // ADR-008
}

// New creates a Nexus Client.
func New(nexusAddr string) *Client {
	return &Client{
		baseURL:    nexusAddr,
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
		req.Header.Set(canon.ServiceTokenHeader, c.serviceToken) // ADR-008
	}
	return c.httpClient.Do(req)
}

// Ping checks whether the Nexus daemon is reachable.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("nexus: ping build request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("nexus unreachable at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("nexus health: HTTP %d", resp.StatusCode)
	}
	return nil
}

// GetProject fetches a single project by ID from the Nexus registry.
// Returns nil, nil when the project is not found (404).
func (c *Client) GetProject(ctx context.Context, id string) (*Project, error) {
	resp, err := c.get(ctx, "/projects/"+id)
	if err != nil {
		return nil, fmt.Errorf("nexus: GET /projects/%s: %w", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nexus: GET /projects/%s: HTTP %d", id, resp.StatusCode)
	}

	var envelope struct {
		OK   bool            `json:"ok"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("nexus: decode project response: %w", err)
	}
	if !envelope.OK {
		return nil, fmt.Errorf("nexus: API returned ok=false for project %s", id)
	}

	var project Project
	if err := json.Unmarshal(envelope.Data, &project); err != nil {
		return nil, fmt.Errorf("nexus: decode project: %w", err)
	}
	return &project, nil
}

// GetAllProjects fetches all projects from the Nexus registry.
func (c *Client) GetAllProjects(ctx context.Context) ([]*Project, error) {
	resp, err := c.get(ctx, "/projects")
	if err != nil {
		return nil, fmt.Errorf("nexus: GET /projects: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nexus: GET /projects: HTTP %d", resp.StatusCode)
	}

	var envelope struct {
		OK   bool       `json:"ok"`
		Data []*Project `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("nexus: decode projects: %w", err)
	}
	if !envelope.OK {
		return nil, fmt.Errorf("nexus: GET /projects returned ok=false")
	}
	return envelope.Data, nil
}
