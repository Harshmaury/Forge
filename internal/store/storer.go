// @forge-project: forge
// @forge-path: internal/store/storer.go
// FG-H-03: WithWorkflowTransaction added for atomic workflow creation.
//
// Storer is the read/write contract for the Forge workflow database.
// *Store satisfies this interface. Tests supply a mock.
//
// Phase 2: Workflow + WorkflowStep
// Phase 3: Trigger (event-to-workflow mapping)
package store

import "time"

// ── PHASE 2 TYPES ─────────────────────────────────────────────────────────────

// Workflow is a named, reusable sequence of commands.
type Workflow struct {
	ID          string
	Name        string
	Description string
	Trigger     string // "manual" | "event"
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// WorkflowStep is one command within a workflow, ordered by position.
type WorkflowStep struct {
	ID         int64
	WorkflowID string
	Position   int
	Intent     string
	Target     string
	Parameters map[string]string
}

// ── PHASE 3 TYPES ─────────────────────────────────────────────────────────────

// Trigger maps a workspace event topic to a stored workflow.
// When a workspace event arrives and matches the trigger's filter,
// the linked workflow is executed automatically (ADR-007).
type Trigger struct {
	ID         string    // UUID
	Event      string    // workspace topic (e.g. "workspace.file.modified")
	WorkflowID string    // must reference an existing workflow
	FilterExt  string    // optional: file extension filter (e.g. ".go")
	FilterProj string    // optional: Atlas project ID filter
	FilterDir  string    // optional: directory path prefix filter
	Enabled    bool      // disabled triggers are never fired
	CreatedAt  time.Time
}

// ── PHASE 4 TYPES ─────────────────────────────────────────────────────────────

// ExecutionRecord is a persisted record of a command execution (ADR-010).
type ExecutionRecord struct {
	ID         string    // UUID
	CommandID  string    // Command.ID
	Intent     string
	Target     string
	TraceID    string
	Status     string    // "success" | "failure" | "denied"
	Output     string
	Error      string
	DurationMS int64
	StartedAt  time.Time
	FinishedAt time.Time
}

// ── STORER INTERFACE ──────────────────────────────────────────────────────────

// Storer is the Forge workflow store contract.
type Storer interface {
	// ── Lifecycle ──────────────────────────────────────────────
	Close() error

	// ── Workflows (Phase 2) ────────────────────────────────────
	CreateWorkflow(w *Workflow) error
	GetWorkflow(id string) (*Workflow, error)
	GetAllWorkflows() ([]*Workflow, error)
	DeleteWorkflow(id string) error

	// WithWorkflowTransaction executes fn inside a SQLite transaction.
	WithWorkflowTransaction(fn func() error) error

	// ── Steps (Phase 2) ────────────────────────────────────────
	AddStep(s *WorkflowStep) error
	GetSteps(workflowID string) ([]*WorkflowStep, error)
	DeleteSteps(workflowID string) error

	// ── Triggers (Phase 3) ─────────────────────────────────────
	CreateTrigger(t *Trigger) error
	GetTrigger(id string) (*Trigger, error)
	GetAllTriggers() ([]*Trigger, error)
	GetEnabledTriggersByEvent(event string) ([]*Trigger, error)
	DeleteTrigger(id string) error

	// ── Execution history (Phase 4 / ADR-010) ──────────────────
	LogExecution(r *ExecutionRecord) error
	GetHistory(limit int) ([]*ExecutionRecord, error)
	GetHistoryByTrace(traceID string) ([]*ExecutionRecord, error)
}
