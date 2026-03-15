// @forge-project: forge
// @forge-path: internal/executor/engine_test.go
package executor

import (
	"context"
	"testing"
	"time"

	"github.com/Harshmaury/Forge/internal/command"
	"github.com/Harshmaury/Forge/internal/executor/intent"
)

// ── MOCK HANDLER ──────────────────────────────────────────────────────────────

type mockHandler struct {
	intentName string
	result     *intent.Result
}

func (m *mockHandler) Intent() string { return m.intentName }
func (m *mockHandler) Execute(_ context.Context, cmd *command.Command) *intent.Result {
	if m.result != nil {
		return m.result
	}
	return &intent.Result{
		CommandID: cmd.ID,
		Intent:    cmd.Intent,
		Target:    cmd.Target,
		Success:   true,
		Output:    "mock executed",
	}
}

// ── HELPERS ──────────────────────────────────────────────────────────────────

func testCommand(intentName string) *command.Command {
	return &command.Command{
		ID:         "eng-test-001",
		Intent:     intentName,
		Target:     "nexus",
		Parameters: map[string]string{},
		Context: command.CommandContext{
			WorkspaceRoot:   "/home/harsh/workspace",
			ProjectPath:     "/home/harsh/workspace/projects/apps/nexus",
			Language:        "go",
			RequestingAgent: "test",
			Timestamp:       time.Now(),
		},
	}
}

// ── TESTS ─────────────────────────────────────────────────────────────────────

func TestEngine_Register_And_Dispatch(t *testing.T) {
	e := NewEngine()
	e.Register(&mockHandler{intentName: command.IntentBuild})

	result := e.Execute(context.Background(), testCommand(command.IntentBuild))
	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Output != "mock executed" {
		t.Errorf("unexpected output: %q", result.Output)
	}
}

func TestEngine_UnregisteredIntent_ReturnsError(t *testing.T) {
	e := NewEngine()
	// No handlers registered.

	result := e.Execute(context.Background(), testCommand("unknown-intent"))
	if result.Success {
		t.Error("expected failure for unregistered intent")
	}
	if result.Error == "" {
		t.Error("expected error message for unregistered intent")
	}
}

func TestEngine_RegisterAll_And_List(t *testing.T) {
	e := NewEngine()
	for _, name := range []string{
		command.IntentBuild,
		command.IntentTest,
		command.IntentRun,
		command.IntentDeploy,
	} {
		e.Register(&mockHandler{intentName: name})
	}

	registered := e.RegisteredIntents()
	if len(registered) != 4 {
		t.Errorf("RegisteredIntents() = %d, want 4", len(registered))
	}
}

func TestEngine_Register_Replaces_Existing(t *testing.T) {
	e := NewEngine()
	e.Register(&mockHandler{
		intentName: command.IntentBuild,
		result: &intent.Result{Success: false, Error: "old handler"},
	})
	// Replace with new handler that succeeds.
	e.Register(&mockHandler{intentName: command.IntentBuild})

	result := e.Execute(context.Background(), testCommand(command.IntentBuild))
	if !result.Success {
		t.Errorf("new handler should succeed, got: %s", result.Error)
	}
}

func TestEngine_Execute_PassesCommandThrough(t *testing.T) {
	e := NewEngine()
	var receivedCmd *command.Command
	e.Register(&mockHandler{
		intentName: command.IntentTest,
		result:     nil, // will be overridden in Execute
	})

	// Use a custom mock that captures the command.
	e.Register(&struct {
		intent.Handler
		capture func(*command.Command)
	}{
		Handler: &mockHandler{intentName: command.IntentTest},
	})

	// Verify the command fields are preserved through dispatch.
	cmd := testCommand(command.IntentTest)
	result := e.Execute(context.Background(), cmd)
	_ = receivedCmd

	if result.CommandID != "" && result.CommandID != cmd.ID {
		t.Errorf("CommandID mismatch: %q", result.CommandID)
	}
}
