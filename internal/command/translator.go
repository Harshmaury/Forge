// @forge-project: forge
// @forge-path: internal/command/translator.go
// Translator converts a RawCommandRequest into a validated Command object.
//
// Translation steps:
//  1. Assign ID — use caller-supplied ID or generate a UUID
//  2. Copy intent, target, parameters verbatim
//  3. Populate context — use caller-supplied context or fill defaults
//  4. Validate — return ValidationError if any field fails
//
// The translator is the single entry point for all raw input.
// CLI flags, HTTP bodies, and Phase 3 event triggers all pass through here.
// The executor never receives a Command that has not been translated.
package command

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ── TRANSLATOR ────────────────────────────────────────────────────────────────

// Translator converts raw input into validated Command objects.
type Translator struct {
	validator     *Validator
	workspaceRoot string
}

// NewTranslator creates a Translator.
// workspaceRoot is used as the default context.workspace_root when
// the caller does not supply a context.
func NewTranslator(workspaceRoot string) *Translator {
	return &Translator{
		validator:     NewValidator(),
		workspaceRoot: workspaceRoot,
	}
}

// Translate converts a RawCommandRequest into a validated Command.
// Returns a ValidationError if any required field is missing or invalid.
func (t *Translator) Translate(raw RawCommandRequest) (*Command, error) {
	cmd := &Command{
		ID:         t.resolveID(raw.ID),
		Intent:     raw.Intent,
		Target:     raw.Target,
		Parameters: t.resolveParameters(raw.Parameters),
		Context:    t.resolveContext(raw.Context),
	}

	if err := t.validator.Validate(cmd); err != nil {
		return nil, fmt.Errorf("translate: %w", err)
	}

	return cmd, nil
}

// TranslateFields is a convenience method for CLI usage where individual
// fields are passed rather than a RawCommandRequest.
func (t *Translator) TranslateFields(
	id, intent, target string,
	parameters map[string]string,
	agent string,
) (*Command, error) {
	var ctx *CommandContext
	if agent != "" {
		c := CommandContext{
			WorkspaceRoot:   t.workspaceRoot,
			RequestingAgent: agent,
			Timestamp:       time.Now().UTC(),
		}
		ctx = &c
	}
	return t.Translate(RawCommandRequest{
		ID:         id,
		Intent:     intent,
		Target:     target,
		Parameters: parameters,
		Context:    ctx,
	})
}

// ── FIELD RESOLVERS ───────────────────────────────────────────────────────────

// resolveID returns the caller ID or generates a UUID.
func (t *Translator) resolveID(callerID string) string {
	if callerID != "" {
		return callerID
	}
	return uuid.New().String()
}

// resolveParameters returns a non-nil parameter map.
func (t *Translator) resolveParameters(params map[string]string) map[string]string {
	if params != nil {
		return params
	}
	return map[string]string{}
}

// resolveContext fills missing context fields with workspace defaults.
func (t *Translator) resolveContext(ctx *CommandContext) CommandContext {
	if ctx != nil {
		resolved := *ctx
		if resolved.WorkspaceRoot == "" {
			resolved.WorkspaceRoot = t.workspaceRoot
		}
		if resolved.RequestingAgent == "" {
			resolved.RequestingAgent = "unknown"
		}
		if resolved.Timestamp.IsZero() {
			resolved.Timestamp = time.Now().UTC()
		}
		return resolved
	}
	return CommandContext{
		WorkspaceRoot:   t.workspaceRoot,
		RequestingAgent: "unknown",
		Timestamp:       time.Now().UTC(),
	}
}
