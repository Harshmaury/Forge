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
	Schedule   string    // @every <dur> | @hourly | @daily (empty = event trigger)
	CreatedAt  time.Time
}

// ── PHASE 4 TYPES ─────────────────────────────────────────────────────────────

// PreflightSnapshot is the Atlas context captured at authorization time (ADR-021).
// Serialised as JSON into execution_history.preflight_snapshot_json (migration v4).
// Zero value is safe — represents a record written before ADR-021 was applied.
type PreflightSnapshot struct {
	AtlasQueried  bool     `json:"atlas_queried"`
	ProjectFound  bool     `json:"project_found"`
	ProjectID     string   `json:"project_id"`
	ProjectStatus string   `json:"project_status"`
	Capabilities  []string `json:"capabilities"`
	DependsOn     []string `json:"depends_on"`
	SnapshotAt    string   `json:"snapshot_at"` // RFC3339Nano UTC
}

// ExecutionRecord is a persisted record of a command execution (ADR-010).
// ADR-021: PreflightSnapshot field added — captures Atlas state at auth time.
// json tags use snake_case for consistent API output to all observer consumers.
type ExecutionRecord struct {
	ID                string            `json:"id"`
	CommandID         string            `json:"command_id"`
	Intent            string            `json:"intent"`
	Target            string            `json:"target"`
	TraceID           string            `json:"trace_id"`
	Status            string            `json:"status"`
	Output            string            `json:"output"`
	Error             string            `json:"error"`
	DurationMS        int64             `json:"duration_ms"`
	StartedAt         time.Time         `json:"started_at"`
	FinishedAt        time.Time         `json:"finished_at"`
	PreflightSnapshot PreflightSnapshot `json:"preflight_snapshot"`
}


// ── DEDUP TYPES (CW-5) ────────────────────────────────────────────────────────

// DedupTTL is the window during which a command_id is considered a duplicate.
// Covers all reasonable CLI retry intervals (default: 5s–30s exponential backoff).
const DedupTTL = 5 * 60 // 300 seconds

// DedupRecord is a stored idempotency key for a completed command execution.
// ResultJSON is the JSON-serialised command.ExecutionResult returned to the
// original caller — replayed verbatim to the duplicate caller (HTTP 409).
type DedupRecord struct {
	CommandID  string    // the caller-supplied command.Command.ID
	ResultJSON string    // JSON of command.ExecutionResult
	ExpiresAt  time.Time // UTC — record is purged after this time
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
	GetEnabledCronTriggers() ([]*Trigger, error)
	DeleteTrigger(id string) error

	// ── Execution history (Phase 4 / ADR-010) ──────────────────
	LogExecution(r *ExecutionRecord) error
	GetHistory(limit int) ([]*ExecutionRecord, error)
	GetHistoryByTrace(traceID string) ([]*ExecutionRecord, error)

	// ── Idempotency dedup (CW-5) ──────────────────────────────────
	// SetDedupRecord stores a completed result keyed by command_id.
	// Replaces any existing record for the same command_id (upsert).
	SetDedupRecord(r *DedupRecord) error
	// GetDedupRecord returns the cached result for command_id if it
	// exists and has not expired. Returns nil, nil when not found.
	GetDedupRecord(commandID string) (*DedupRecord, error)
}
