// @forge-project: forge
// @forge-path: internal/executor/engine.go
// Package executor dispatches validated Commands to registered intent handlers.
//
// The engine is the boundary between Forge's translation layer and its
// execution layer. It never receives raw input — every command it sees
// has been translated, validated, and context-enriched.
//
// Registration pattern:
//   engine := NewEngine()
//   engine.Register(intent.NewBuildHandler())
//   engine.Register(intent.NewTestHandler())
//
// Execution:
//   result := engine.Execute(ctx, cmd)
package executor

import (
	"context"
	"fmt"

	"github.com/Harshmaury/Forge/internal/command"
	"github.com/Harshmaury/Forge/internal/executor/intent"
)

// ── ENGINE ────────────────────────────────────────────────────────────────────

// Engine dispatches Commands to registered intent handlers.
type Engine struct {
	handlers map[string]intent.Handler
}

// NewEngine creates an Engine with no registered handlers.
// Call Register for each supported intent before use.
func NewEngine() *Engine {
	return &Engine{handlers: make(map[string]intent.Handler)}
}

// Register adds a handler to the engine.
// If a handler for the same intent is already registered, it is replaced.
func (e *Engine) Register(h intent.Handler) {
	e.handlers[h.Intent()] = h
}

// RegisteredIntents returns the list of intent names with registered handlers.
func (e *Engine) RegisteredIntents() []string {
	intents := make([]string, 0, len(e.handlers))
	for name := range e.handlers {
		intents = append(intents, name)
	}
	return intents
}

// Execute dispatches a Command to the appropriate handler.
// Returns an error result if no handler is registered for the intent.
// The command must be validated and context-enriched before calling Execute.
func (e *Engine) Execute(ctx context.Context, cmd *command.Command) *intent.Result {
	h, ok := e.handlers[cmd.Intent]
	if !ok {
		return &intent.Result{
			CommandID: cmd.ID,
			Intent:    cmd.Intent,
			Target:    cmd.Target,
			Success:   false,
			Error:     fmt.Sprintf("no handler registered for intent %q", cmd.Intent),
		}
	}
	return h.Execute(ctx, cmd)
}
