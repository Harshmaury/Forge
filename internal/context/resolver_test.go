// @forge-project: forge
// @forge-path: internal/context/resolver_test.go
package context

import (
	"context"
	"errors"
	"log"
	"os"
	"testing"
	"time"

	"github.com/Harshmaury/Forge/internal/atlas"
	"github.com/Harshmaury/Forge/internal/command"
	"github.com/Harshmaury/Forge/internal/nexus"
)

// ── MOCK CLIENTS ─────────────────────────────────────────────────────────────

type mockNexus struct {
	project *nexus.Project
	err     error
}

func (m *mockNexus) GetProject(_ context.Context, id string) (*nexus.Project, error) {
	return m.project, m.err
}

type mockAtlas struct {
	project      *atlas.ProjectDetail
	projectErr   error
	wsCtx        *atlas.WorkspaceContext
	wsCtxErr     error
}

func (m *mockAtlas) GetProject(_ context.Context, id string) (*atlas.ProjectDetail, error) {
	return m.project, m.projectErr
}

func (m *mockAtlas) GetWorkspaceContext(_ context.Context) (*atlas.WorkspaceContext, error) {
	return m.wsCtx, m.wsCtxErr
}

// ── HELPERS ──────────────────────────────────────────────────────────────────

func baseCommand() *command.Command {
	return &command.Command{
		ID:         "test-001",
		Intent:     command.IntentBuild,
		Target:     "nexus",
		Parameters: map[string]string{},
		Context: command.CommandContext{
			WorkspaceRoot:   "/home/harsh/workspace",
			RequestingAgent: "cli",
			Timestamp:       time.Now(),
		},
	}
}

func newResolver(n NexusClient, a AtlasClient) *Resolver {
	return NewResolver(n, a, log.New(os.Stderr, "[test] ", 0))
}

// ── TESTS ─────────────────────────────────────────────────────────────────────

func TestResolveContext_FillsProjectPath(t *testing.T) {
	cmd := baseCommand()
	cmd.Context.ProjectPath = "" // missing — should be filled

	n := &mockNexus{project: &nexus.Project{ID: "nexus", Path: "/ignored"}}
	a := &mockAtlas{
		project: &atlas.ProjectDetail{
			ID:       "nexus",
			Path:     "/home/harsh/workspace/projects/apps/nexus",
			Language: "go",
		},
		wsCtx: &atlas.WorkspaceContext{WorkspaceRoot: "/home/harsh/workspace"},
	}

	r := newResolver(n, a)
	enriched := r.ResolveContext(context.Background(), cmd)

	if enriched.Context.ProjectPath != "/home/harsh/workspace/projects/apps/nexus" {
		t.Errorf("ProjectPath = %q, want /home/harsh/workspace/projects/apps/nexus",
			enriched.Context.ProjectPath)
	}
}

func TestResolveContext_FillsLanguage(t *testing.T) {
	cmd := baseCommand()
	cmd.Context.Language = "" // missing

	n := &mockNexus{project: &nexus.Project{ID: "nexus"}}
	a := &mockAtlas{
		project: &atlas.ProjectDetail{ID: "nexus", Language: "go"},
		wsCtx:   &atlas.WorkspaceContext{WorkspaceRoot: "/home/harsh/workspace"},
	}

	r := newResolver(n, a)
	enriched := r.ResolveContext(context.Background(), cmd)

	if enriched.Context.Language != "go" {
		t.Errorf("Language = %q, want go", enriched.Context.Language)
	}
}

func TestResolveContext_FillsWorkspaceRoot(t *testing.T) {
	cmd := baseCommand()
	cmd.Context.WorkspaceRoot = "" // missing

	n := &mockNexus{project: &nexus.Project{ID: "nexus"}}
	a := &mockAtlas{
		wsCtx: &atlas.WorkspaceContext{WorkspaceRoot: "/home/harsh/workspace"},
	}

	r := newResolver(n, a)
	enriched := r.ResolveContext(context.Background(), cmd)

	if enriched.Context.WorkspaceRoot != "/home/harsh/workspace" {
		t.Errorf("WorkspaceRoot = %q, want /home/harsh/workspace",
			enriched.Context.WorkspaceRoot)
	}
}

func TestResolveContext_PreservesCallerSuppliedFields(t *testing.T) {
	cmd := baseCommand()
	cmd.Context.ProjectPath = "/custom/path"
	cmd.Context.Language = "rust"

	n := &mockNexus{project: &nexus.Project{ID: "nexus"}}
	a := &mockAtlas{
		project: &atlas.ProjectDetail{ID: "nexus", Path: "/atlas/path", Language: "go"},
		wsCtx:   &atlas.WorkspaceContext{WorkspaceRoot: "/home/harsh/workspace"},
	}

	r := newResolver(n, a)
	enriched := r.ResolveContext(context.Background(), cmd)

	// Caller-supplied values must not be overwritten.
	if enriched.Context.ProjectPath != "/custom/path" {
		t.Errorf("ProjectPath overwritten: got %q, want /custom/path",
			enriched.Context.ProjectPath)
	}
	if enriched.Context.Language != "rust" {
		t.Errorf("Language overwritten: got %q, want rust", enriched.Context.Language)
	}
}

func TestResolveContext_DegradeGracefullyWhenAtlasDown(t *testing.T) {
	cmd := baseCommand()
	// Atlas is down — both calls return errors.
	n := &mockNexus{project: &nexus.Project{ID: "nexus"}}
	a := &mockAtlas{
		projectErr: errors.New("connection refused"),
		wsCtxErr:   errors.New("connection refused"),
	}

	r := newResolver(n, a)
	// Must not panic or return nil.
	enriched := r.ResolveContext(context.Background(), cmd)
	if enriched == nil {
		t.Fatal("expected non-nil result even when Atlas is down")
	}
	// Original fields preserved.
	if enriched.Context.WorkspaceRoot != cmd.Context.WorkspaceRoot {
		t.Errorf("WorkspaceRoot changed unexpectedly")
	}
}

func TestResolveContext_DegradeGracefullyWhenNexusDown(t *testing.T) {
	cmd := baseCommand()
	n := &mockNexus{err: errors.New("connection refused")}
	a := &mockAtlas{
		project: &atlas.ProjectDetail{ID: "nexus", Path: "/path", Language: "go"},
		wsCtx:   &atlas.WorkspaceContext{WorkspaceRoot: "/home/harsh/workspace"},
	}

	r := newResolver(n, a)
	enriched := r.ResolveContext(context.Background(), cmd)
	if enriched == nil {
		t.Fatal("expected non-nil result even when Nexus is down")
	}
}

func TestResolveContext_SetsTimestampIfZero(t *testing.T) {
	cmd := baseCommand()
	cmd.Context.Timestamp = time.Time{} // zero

	n := &mockNexus{project: &nexus.Project{ID: "nexus"}}
	a := &mockAtlas{wsCtx: &atlas.WorkspaceContext{WorkspaceRoot: "/home/harsh/workspace"}}

	r := newResolver(n, a)
	enriched := r.ResolveContext(context.Background(), cmd)

	if enriched.Context.Timestamp.IsZero() {
		t.Error("Timestamp should be set after resolution")
	}
}

func TestValidateTarget_Found(t *testing.T) {
	n := &mockNexus{project: &nexus.Project{ID: "nexus", Name: "Nexus"}}
	a := &mockAtlas{}
	r := newResolver(n, a)

	if err := r.ValidateTarget(context.Background(), "nexus"); err != nil {
		t.Errorf("expected no error for known target, got: %v", err)
	}
}

func TestValidateTarget_NotFound(t *testing.T) {
	n := &mockNexus{project: nil} // 404
	a := &mockAtlas{}
	r := newResolver(n, a)

	if err := r.ValidateTarget(context.Background(), "unknown-project"); err == nil {
		t.Error("expected error for unknown target, got nil")
	}
}

func TestValidateTarget_NexusDown(t *testing.T) {
	n := &mockNexus{err: errors.New("connection refused")}
	a := &mockAtlas{}
	r := newResolver(n, a)

	if err := r.ValidateTarget(context.Background(), "nexus"); err == nil {
		t.Error("expected error when Nexus is unreachable")
	}
}
