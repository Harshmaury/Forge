// @forge-project: forge
// @forge-path: internal/context/resolver.go
// Package context enriches a Command's context field with live workspace data.
//
// Resolution order:
//  1. Use any fields already present in cmd.Context (caller-supplied)
//  2. Fill missing project_path + language from Atlas GET /workspace/project/:id
//  3. Fill missing workspace_root from Atlas GET /workspace/context
//  4. Verify target exists in Nexus GET /projects/:id
//
// Graceful degradation:
//   If Atlas is unreachable, return cmd as-is — execution continues
//   with whatever context the caller supplied.
//   If Nexus is unreachable, log a warning and continue — the target
//   existence check is advisory, not a hard gate in Phase 1.
package context

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Harshmaury/Forge/internal/atlas"
	"github.com/Harshmaury/Forge/internal/command"
	"github.com/Harshmaury/Forge/internal/nexus"
)

// ── INTERFACES ────────────────────────────────────────────────────────────────

// NexusClient is the subset of nexus.Client used by the Resolver.
type NexusClient interface {
	GetProject(ctx context.Context, id string) (*nexus.Project, error)
}

// AtlasClient is the subset of atlas.Client used by the Resolver.
type AtlasClient interface {
	GetProject(ctx context.Context, id string) (*atlas.ProjectDetail, error)
	GetWorkspaceContext(ctx context.Context) (*atlas.WorkspaceContext, error)
}

// ── RESOLVER ─────────────────────────────────────────────────────────────────

// Resolver enriches command context from Atlas and Nexus.
type Resolver struct {
	nexus  NexusClient
	atlas  AtlasClient
	logger *log.Logger
}

// NewResolver creates a Resolver.
func NewResolver(n NexusClient, a AtlasClient, logger *log.Logger) *Resolver {
	return &Resolver{nexus: n, atlas: a, logger: logger}
}

// ResolveContext enriches cmd.Context with data from Atlas and Nexus.
// Returns the enriched command. Never returns an error — degradation is silent.
func (r *Resolver) ResolveContext(ctx context.Context, cmd *command.Command) *command.Command {
	enriched := *cmd // shallow copy — safe because Context is a value type

	// ── Step 1: Fill workspace_root from Atlas if missing ────────────────────
	if enriched.Context.WorkspaceRoot == "" {
		if wsCtx, err := r.atlas.GetWorkspaceContext(ctx); err == nil && wsCtx != nil {
			enriched.Context.WorkspaceRoot = wsCtx.WorkspaceRoot
		} else if err != nil {
			r.logger.Printf("WARNING: atlas workspace context unavailable: %v", err)
		}
	}

	// ── Step 2: Fill project_path + language from Atlas ───────────────────────
	if enriched.Context.ProjectPath == "" || enriched.Context.Language == "" {
		if proj, err := r.atlas.GetProject(ctx, cmd.Target); err == nil && proj != nil {
			if enriched.Context.ProjectPath == "" {
				enriched.Context.ProjectPath = proj.Path
			}
			if enriched.Context.Language == "" {
				enriched.Context.Language = proj.Language
			}
		} else if err != nil {
			r.logger.Printf("WARNING: atlas project %s unavailable: %v", cmd.Target, err)
		}
	}

	// ── Step 3: Verify target exists in Nexus (advisory) ─────────────────────
	if _, err := r.nexus.GetProject(ctx, cmd.Target); err != nil {
		r.logger.Printf("WARNING: nexus project %s lookup failed: %v", cmd.Target, err)
		// Do not block execution — Nexus may be temporarily unavailable.
	}

	// ── Step 4: Ensure timestamp is set ──────────────────────────────────────
	if enriched.Context.Timestamp.IsZero() {
		enriched.Context.Timestamp = time.Now().UTC()
	}

	return &enriched
}

// ValidateTarget checks that the command target exists in the Nexus registry.
// Returns an error if the target is not found. Used as a hard gate when
// Nexus is confirmed reachable.
func (r *Resolver) ValidateTarget(ctx context.Context, target string) error {
	proj, err := r.nexus.GetProject(ctx, target)
	if err != nil {
		return fmt.Errorf("nexus unreachable — cannot validate target %q: %w", target, err)
	}
	if proj == nil {
		return fmt.Errorf("target %q not found in Nexus project registry", target)
	}
	return nil
}
