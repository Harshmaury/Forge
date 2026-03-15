// @forge-project: forge
// @forge-path: internal/store/storer.go
// Storer is the read/write contract for the Forge workflow database.
// *Store satisfies this interface. Tests supply a mock.
//
// Phase 2 additions:
//   Workflow — named sequence of commands
//   WorkflowStep — one command within a workflow
package store

import "time"

// ── TYPES ─────────────────────────────────────────────────────────────────────

// Workflow is a named, reusable sequence of commands.
// Stored in SQLite; executed by the workflow executor.
type Workflow struct {
	ID          string    // UUID
	Name        string    // human-readable name (e.g. "full-deploy")
	Description string    // optional description
	Trigger     string    // "manual" (Phase 2) | "event" (Phase 3)
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// WorkflowStep is one command within a workflow, ordered by position.
type WorkflowStep struct {
	ID         int64
	WorkflowID string
	Position   int    // 1-based execution order
	Intent     string // command intent (build, test, run, deploy)
	Target     string // project or service ID
	Parameters map[string]string
}

// ── STORER INTERFACE ──────────────────────────────────────────────────────────

// Storer is the Forge workflow store contract.
type Storer interface {
	// ── Lifecycle ──────────────────────────────────────────────
	Close() error

	// ── Workflows ──────────────────────────────────────────────
	CreateWorkflow(w *Workflow) error
	GetWorkflow(id string) (*Workflow, error)
	GetAllWorkflows() ([]*Workflow, error)
	DeleteWorkflow(id string) error

	// ── Steps ──────────────────────────────────────────────────
	AddStep(s *WorkflowStep) error
	GetSteps(workflowID string) ([]*WorkflowStep, error)
	DeleteSteps(workflowID string) error
}
