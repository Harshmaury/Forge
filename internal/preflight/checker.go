// @forge-project: forge
// @forge-path: internal/preflight/checker.go
// Package preflight implements pre-execution validation for Forge Phase 4.
//
// ADR-010: before the executor sees a command, PreflightChecker queries
// Atlas GET /graph/services and verifies the target project is verified.
//
// ADR-021: Check() now returns a PreflightSnapshot alongside the permit/deny
// decision. The snapshot captures the Atlas response at the moment of the
// check — it is frozen before engine.Execute() is called, eliminating the
// RC-001 race where Atlas may rescan between check and execution log.
//
// ADR-006 constraint: Atlas provides facts. Forge decides policy.
// Fail-open: if Atlas is unreachable, the check is skipped with a WARNING.
package preflight

import (
	"context"
	"fmt"
	"log"
	"time"

	atlasclient "github.com/Harshmaury/Forge/internal/atlas"
)

// PreflightSnapshot is an immutable record of the Atlas context at the
// moment of preflight authorization (ADR-021).
//
// Captured once in Check() and passed by value through the execution
// pipeline — never re-queried between check and log. This eliminates
// the RC-001 race where Atlas rescans between preflight and history write.
type PreflightSnapshot struct {
	AtlasQueried  bool      `json:"atlas_queried"`   // false = Atlas unreachable
	ProjectFound  bool      `json:"project_found"`   // false = not in verified graph
	ProjectID     string    `json:"project_id"`      // target ID as returned by Atlas
	ProjectStatus string    `json:"project_status"`  // "verified" | "" if not found
	Capabilities  []string  `json:"capabilities"`    // from nexus.yaml
	DependsOn     []string  `json:"depends_on"`      // from nexus.yaml
	SnapshotAt    time.Time `json:"snapshot_at"`     // UTC time of the check
}

// Result is the outcome of a preflight check.
type Result struct {
	Permitted bool
	Reason    string             // deny reason, empty if permitted
	Snapshot  PreflightSnapshot  // always populated, even on deny (ADR-021)
}

// Checker performs pre-execution validation against the Atlas graph.
type Checker struct {
	atlas  *atlasclient.Client
	logger *log.Logger
}

// NewChecker creates a Checker.
func NewChecker(atlas *atlasclient.Client, logger *log.Logger) *Checker {
	if logger == nil {
		logger = log.Default()
	}
	return &Checker{atlas: atlas, logger: logger}
}

// Check validates the target exists in the Atlas verified graph.
// Never returns an error — fails open if Atlas is unreachable.
// The returned Result.Snapshot is frozen at call time (ADR-021).
func (c *Checker) Check(ctx context.Context, target string) *Result {
	snap := PreflightSnapshot{SnapshotAt: time.Now().UTC()}

	services, err := c.atlas.GetVerifiedServices(ctx)
	if err != nil {
		c.logger.Printf("WARNING: preflight check skipped — Atlas unavailable: %v", err)
		snap.AtlasQueried = false
		return &Result{
			Permitted: true,
			Reason:    "atlas unavailable — check skipped",
			Snapshot:  snap,
		}
	}

	snap.AtlasQueried = true
	return c.findTarget(target, services, snap)
}

// findTarget searches the verified service list for the target project.
// Extracted from Check() to keep both functions under 40 lines.
func (c *Checker) findTarget(
	target string,
	services []*atlasclient.ProjectDetail,
	snap PreflightSnapshot,
) *Result {
	for _, svc := range services {
		if svc.ID != target {
			continue
		}
		snap.ProjectFound  = true
		snap.ProjectID     = svc.ID
		snap.ProjectStatus = svc.Status
		snap.Capabilities  = svc.Capabilities
		snap.DependsOn     = svc.DependsOn
		return &Result{Permitted: true, Snapshot: snap}
	}

	return &Result{
		Permitted: false,
		Reason: fmt.Sprintf(
			"project %q not found in Atlas verified graph — add nexus.yaml to register it",
			target,
		),
		Snapshot: snap,
	}
}
