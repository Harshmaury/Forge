// @forge-project: forge
// @forge-path: internal/executor/intent/handler.go
// Package intent defines the Handler interface and shared execution types.
// Each intent (build, test, run, deploy) implements Handler.
// The executor dispatches to the correct handler by intent name.
package intent

import (
	"context"
	"time"

	"github.com/Harshmaury/Forge/internal/command"
)

// ── HANDLER INTERFACE ─────────────────────────────────────────────────────────

// Handler executes a single intent.
// Every intent handler must implement this interface.
type Handler interface {
	// Intent returns the intent name this handler serves.
	Intent() string

	// Execute runs the intent and returns a Result.
	// cmd is guaranteed to be validated and context-enriched.
	Execute(ctx context.Context, cmd *command.Command) *Result
}

// ── RESULT ────────────────────────────────────────────────────────────────────

// Result is the output of an intent execution.
type Result struct {
	CommandID string
	Intent    string
	Target    string
	Success   bool
	Output    string
	Error     string
	Duration  time.Duration
	Metadata  map[string]string
}

// ToExecutionResult converts a Result to a command.ExecutionResult for the API layer.
func (r *Result) ToExecutionResult() *command.ExecutionResult {
	return &command.ExecutionResult{
		CommandID: r.CommandID,
		Intent:    r.Intent,
		Target:    r.Target,
		Success:   r.Success,
		Output:    r.Output,
		Error:     r.Error,
		Duration:  r.Duration.String(),
		Metadata:  r.Metadata,
	}
}

// ── LANGUAGE BUILD COMMANDS ───────────────────────────────────────────────────

// languageBuildCmd maps a project language to its build command + args.
var languageBuildCmd = map[string][]string{
	"go":     {"go", "build", "./..."},
	"python": {"python3", "-m", "py_compile"},
	"node":   {"npm", "run", "build"},
	"rust":   {"cargo", "build"},
	"dotnet": {"dotnet", "build"},
}

// languageTestCmd maps a project language to its test command + args.
var languageTestCmd = map[string][]string{
	"go":     {"go", "test", "./...", "-count=1"},
	"python": {"python3", "-m", "pytest"},
	"node":   {"npm", "test"},
	"rust":   {"cargo", "test"},
	"dotnet": {"dotnet", "test"},
}

// BuildCmdForLanguage returns the build command for a language, or nil if unknown.
func BuildCmdForLanguage(lang string) []string {
	return languageBuildCmd[lang]
}

// TestCmdForLanguage returns the test command for a language, or nil if unknown.
func TestCmdForLanguage(lang string) []string {
	return languageTestCmd[lang]
}
