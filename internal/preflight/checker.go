// @forge-project: forge
// @forge-path: internal/preflight/checker.go
// Package preflight implements pre-execution validation for Forge Phase 4.
//
// ADR-010: before the executor sees a command, PreflightChecker queries
// Atlas GET /graph/services and verifies the target project is verified.
//
// ADR-006 constraint: Atlas provides facts. Forge decides policy.
// Fail-open: if Atlas is unreachable, the check is skipped with a WARNING.
package preflight

import (
	"context"
	"fmt"
	"log"

	atlasclient "github.com/Harshmaury/Forge/internal/atlas"
)

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

// Result is the outcome of a preflight check.
type Result struct {
	Permitted bool
	Reason    string                     // deny reason, empty if permitted
	Project   *atlasclient.ProjectDetail // non-nil when project was found
}

// Check validates the target exists in the Atlas verified graph.
// Never returns an error — fails open if Atlas is unreachable.
func (c *Checker) Check(ctx context.Context, target string) *Result {
	services, err := c.atlas.GetVerifiedServices(ctx)
	if err != nil {
		c.logger.Printf("WARNING: preflight check skipped — Atlas unavailable: %v", err)
		return &Result{Permitted: true, Reason: "atlas unavailable — check skipped"}
	}

	for _, svc := range services {
		if svc.ID == target {
			return &Result{Permitted: true, Project: svc}
		}
	}

	return &Result{
		Permitted: false,
		Reason:    fmt.Sprintf("project %q not found in Atlas verified graph — add nexus.yaml to register it", target),
	}
}
