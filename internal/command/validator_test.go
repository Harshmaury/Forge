// @forge-project: forge
// @forge-path: internal/command/validator_test.go
package command

import (
	"testing"
	"time"
)

func validCommand() *Command {
	return &Command{
		ID:     "test-id-001",
		Intent: IntentBuild,
		Target: "nexus",
		Parameters: map[string]string{},
		Context: CommandContext{
			WorkspaceRoot:   "/home/harsh/workspace",
			ProjectPath:     "/home/harsh/workspace/projects/apps/nexus",
			Language:        "go",
			RequestingAgent: "cli",
			Timestamp:       time.Now(),
		},
	}
}

func TestValidate_ValidCommand(t *testing.T) {
	v := NewValidator()
	if err := v.Validate(validCommand()); err != nil {
		t.Errorf("expected valid command to pass, got: %v", err)
	}
}

func TestValidate_NilCommand(t *testing.T) {
	v := NewValidator()
	if err := v.Validate(nil); err == nil {
		t.Error("expected error for nil command")
	}
}

func TestValidate_AllIntents(t *testing.T) {
	v := NewValidator()
	for intent := range SupportedIntents {
		cmd := validCommand()
		cmd.Intent = intent
		if err := v.Validate(cmd); err != nil {
			t.Errorf("intent %q should be valid, got: %v", intent, err)
		}
	}
}

func TestValidate_FieldErrors(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*Command)
		wantErr bool
	}{
		{"empty id", func(c *Command) { c.ID = "" }, true},
		{"blank id", func(c *Command) { c.ID = "   " }, true},
		{"empty intent", func(c *Command) { c.Intent = "" }, true},
		{"unknown intent", func(c *Command) { c.Intent = "unknown-action" }, true},
		{"empty target", func(c *Command) { c.Target = "" }, true},
		{"blank target", func(c *Command) { c.Target = "  " }, true},
		{"empty workspace root", func(c *Command) { c.Context.WorkspaceRoot = "" }, true},
		{"empty requesting agent", func(c *Command) { c.Context.RequestingAgent = "" }, true},
		{"nil parameters", func(c *Command) { c.Parameters = nil }, false},
		{"empty parameters", func(c *Command) { c.Parameters = map[string]string{} }, false},
		{"valid parameters", func(c *Command) { c.Parameters = map[string]string{"flag": "true"} }, false},
	}

	v := NewValidator()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := validCommand()
			tc.mutate(cmd)
			err := v.Validate(cmd)
			if tc.wantErr && err == nil {
				t.Errorf("expected validation error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

func TestValidationError_Message(t *testing.T) {
	e := &ValidationError{Field: "intent", Message: "not supported"}
	if e.Error() == "" {
		t.Error("ValidationError.Error() must not be empty")
	}
}
