// @forge-project: forge
// @forge-path: internal/command/model.go
// Package command defines the core Command abstraction (ADR-004).
//
// Every input to Forge — from CLI, HTTP API, or automation trigger —
// is translated into a Command object before the executor sees it.
// The executor never receives raw strings.
//
// ADR-004 schema (fixed — extensions are additive only):
//
//	{
//	  "id":         "<uuid>",
//	  "intent":     "<action name>",
//	  "target":     "<project or service id>",
//	  "parameters": { "<key>": "<value>" },
//	  "context":    { ... ambient metadata ... }
//	}
package command

import "time"

// ── INTENT CONSTANTS ─────────────────────────────────────────────────────────

// Supported Phase 1 intents.
const (
	IntentBuild  = "build"
	IntentTest   = "test"
	IntentRun    = "run"
	IntentDeploy = "deploy"
)

// SupportedIntents is the complete set of Phase 1 intent names.
var SupportedIntents = map[string]bool{
	IntentBuild:  true,
	IntentTest:   true,
	IntentRun:    true,
	IntentDeploy: true,
}

// ── COMMAND ───────────────────────────────────────────────────────────────────

// Command is the core execution unit of Forge (ADR-004).
// All five fields are required. The executor never receives a partial Command.
type Command struct {
	// ID is the unique identifier for this command instance.
	// Used for tracing, idempotency checks, and Phase 2 workflow references.
	// Forge generates a UUID if the caller does not supply one.
	ID string `json:"id"`

	// Intent is the action to perform.
	// Must match a registered intent handler (build, test, run, deploy).
	Intent string `json:"intent"`

	// Target is the project or service ID the action applies to.
	// Validated against the Nexus project registry (ADR-001).
	Target string `json:"target"`

	// Parameters are intent-specific inputs validated per handler.
	Parameters map[string]string `json:"parameters"`

	// Context contains ambient metadata populated from Atlas + Nexus
	// at submission time if not supplied by the caller.
	Context CommandContext `json:"context"`
}

// CommandContext carries ambient workspace metadata at command submission time.
type CommandContext struct {
	WorkspaceRoot   string    `json:"workspace_root"`
	ProjectPath     string    `json:"project_path"`
	Language        string    `json:"language"`
	RequestingAgent string    `json:"requesting_agent"` // "cli" | "http" | "trigger"
	Timestamp       time.Time `json:"timestamp"`
}

// ── RAW INPUT ─────────────────────────────────────────────────────────────────

// RawCommandRequest is the HTTP/CLI input before translation.
// All fields are optional at input — the translator fills gaps and validates.
type RawCommandRequest struct {
	ID         string            `json:"id"`
	Intent     string            `json:"intent"`
	Target     string            `json:"target"`
	Parameters map[string]string `json:"parameters"`
	Context    *CommandContext   `json:"context,omitempty"`
}

// ── RESULT ────────────────────────────────────────────────────────────────────

// ExecutionResult is returned by the executor after running a command.
type ExecutionResult struct {
	CommandID string            `json:"command_id"`
	Intent    string            `json:"intent"`
	Target    string            `json:"target"`
	Success   bool              `json:"success"`
	Output    string            `json:"output"`
	Error     string            `json:"error,omitempty"`
	Duration  string            `json:"duration"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}
