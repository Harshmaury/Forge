// @forge-project: forge
// @forge-path: internal/command/validator.go
// Validator enforces the ADR-004 command schema rules before execution.
// Called by the translator after field population — never by the executor.
package command

import (
	"fmt"
	"strings"
)

// ── VALIDATOR ─────────────────────────────────────────────────────────────────

// Validator checks that a Command satisfies the ADR-004 schema.
type Validator struct{}

// NewValidator creates a Validator.
func NewValidator() *Validator { return &Validator{} }

// ValidationError is returned when one or more fields fail validation.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s — %s", e.Field, e.Message)
}

// Validate checks all five required fields of a Command.
// Returns the first validation error found, or nil if the command is valid.
func (v *Validator) Validate(cmd *Command) error {
	if cmd == nil {
		return &ValidationError{Field: "command", Message: "must not be nil"}
	}
	if err := v.validateID(cmd.ID); err != nil {
		return err
	}
	if err := v.validateIntent(cmd.Intent); err != nil {
		return err
	}
	if err := v.validateTarget(cmd.Target); err != nil {
		return err
	}
	if err := v.validateParameters(cmd.Intent, cmd.Parameters); err != nil {
		return err
	}
	if err := v.validateContext(cmd.Context); err != nil {
		return err
	}
	return nil
}

// ── FIELD VALIDATORS ─────────────────────────────────────────────────────────

func (v *Validator) validateID(id string) error {
	if strings.TrimSpace(id) == "" {
		return &ValidationError{Field: "id", Message: "must not be empty"}
	}
	return nil
}

func (v *Validator) validateIntent(intent string) error {
	if strings.TrimSpace(intent) == "" {
		return &ValidationError{Field: "intent", Message: "must not be empty"}
	}
	if !SupportedIntents[intent] {
		supported := make([]string, 0, len(SupportedIntents))
		for k := range SupportedIntents {
			supported = append(supported, k)
		}
		return &ValidationError{
			Field:   "intent",
			Message: fmt.Sprintf("%q is not a supported intent; supported: %v", intent, supported),
		}
	}
	return nil
}

func (v *Validator) validateTarget(target string) error {
	if strings.TrimSpace(target) == "" {
		return &ValidationError{Field: "target", Message: "must not be empty"}
	}
	return nil
}

// validateParameters checks intent-specific parameter requirements.
// Phase 1 has no mandatory parameters — this is a hook for future validation.
func (v *Validator) validateParameters(intent string, params map[string]string) error {
	if params == nil {
		return nil // parameters are optional
	}
	// Validate parameter keys are non-empty strings.
	for k := range params {
		if strings.TrimSpace(k) == "" {
			return &ValidationError{
				Field:   "parameters",
				Message: "parameter keys must not be empty",
			}
		}
	}
	return nil
}

func (v *Validator) validateContext(ctx CommandContext) error {
	if strings.TrimSpace(ctx.WorkspaceRoot) == "" {
		return &ValidationError{
			Field:   "context.workspace_root",
			Message: "must not be empty",
		}
	}
	if strings.TrimSpace(ctx.RequestingAgent) == "" {
		return &ValidationError{
			Field:   "context.requesting_agent",
			Message: "must not be empty",
		}
	}
	return nil
}
