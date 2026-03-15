// @forge-project: forge
// @forge-path: internal/command/translator_test.go
package command

import (
	"strings"
	"testing"
	"time"
)

const testWorkspaceRoot = "/home/harsh/workspace"

func TestTranslate_ValidInput(t *testing.T) {
	tr := NewTranslator(testWorkspaceRoot)
	raw := RawCommandRequest{
		ID:     "cmd-001",
		Intent: IntentBuild,
		Target: "nexus",
		Parameters: map[string]string{"flag": "verbose"},
		Context: &CommandContext{
			WorkspaceRoot:   testWorkspaceRoot,
			RequestingAgent: "cli",
			Timestamp:       time.Now(),
		},
	}

	cmd, err := tr.Translate(raw)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cmd.ID != "cmd-001" {
		t.Errorf("ID = %q, want cmd-001", cmd.ID)
	}
	if cmd.Intent != IntentBuild {
		t.Errorf("Intent = %q, want build", cmd.Intent)
	}
	if cmd.Target != "nexus" {
		t.Errorf("Target = %q, want nexus", cmd.Target)
	}
	if cmd.Parameters["flag"] != "verbose" {
		t.Errorf("Parameters[flag] = %q, want verbose", cmd.Parameters["flag"])
	}
}

func TestTranslate_GeneratesIDWhenMissing(t *testing.T) {
	tr := NewTranslator(testWorkspaceRoot)
	raw := RawCommandRequest{
		Intent: IntentTest,
		Target: "atlas",
		Context: &CommandContext{
			WorkspaceRoot:   testWorkspaceRoot,
			RequestingAgent: "cli",
		},
	}

	cmd, err := tr.Translate(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.ID == "" {
		t.Error("expected UUID to be generated, got empty string")
	}
	// UUID format: 8-4-4-4-12
	parts := strings.Split(cmd.ID, "-")
	if len(parts) != 5 {
		t.Errorf("generated ID %q does not look like a UUID", cmd.ID)
	}
}

func TestTranslate_FillsContextDefaults(t *testing.T) {
	tr := NewTranslator(testWorkspaceRoot)
	raw := RawCommandRequest{
		ID:     "cmd-002",
		Intent: IntentRun,
		Target: "nexus",
		// No context supplied.
	}

	cmd, err := tr.Translate(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.Context.WorkspaceRoot != testWorkspaceRoot {
		t.Errorf("WorkspaceRoot = %q, want %q", cmd.Context.WorkspaceRoot, testWorkspaceRoot)
	}
	if cmd.Context.RequestingAgent == "" {
		t.Error("RequestingAgent should not be empty after fill")
	}
	if cmd.Context.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero after fill")
	}
}

func TestTranslate_FillsMissingContextFields(t *testing.T) {
	tr := NewTranslator(testWorkspaceRoot)
	raw := RawCommandRequest{
		ID:     "cmd-003",
		Intent: IntentDeploy,
		Target: "forge",
		Context: &CommandContext{
			// WorkspaceRoot and RequestingAgent intentionally empty.
		},
	}

	cmd, err := tr.Translate(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.Context.WorkspaceRoot != testWorkspaceRoot {
		t.Errorf("WorkspaceRoot should default to %q, got %q",
			testWorkspaceRoot, cmd.Context.WorkspaceRoot)
	}
}

func TestTranslate_NilParametersBecomesEmptyMap(t *testing.T) {
	tr := NewTranslator(testWorkspaceRoot)
	raw := RawCommandRequest{
		ID:         "cmd-004",
		Intent:     IntentBuild,
		Target:     "nexus",
		Parameters: nil,
		Context: &CommandContext{
			WorkspaceRoot:   testWorkspaceRoot,
			RequestingAgent: "http",
		},
	}

	cmd, err := tr.Translate(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.Parameters == nil {
		t.Error("Parameters should never be nil after translation")
	}
}

func TestTranslate_InvalidIntentReturnsError(t *testing.T) {
	tr := NewTranslator(testWorkspaceRoot)
	raw := RawCommandRequest{
		ID:     "cmd-005",
		Intent: "destroy",
		Target: "nexus",
		Context: &CommandContext{
			WorkspaceRoot:   testWorkspaceRoot,
			RequestingAgent: "cli",
		},
	}

	_, err := tr.Translate(raw)
	if err == nil {
		t.Error("expected error for unsupported intent, got nil")
	}
	if !strings.Contains(err.Error(), "intent") {
		t.Errorf("error should mention 'intent', got: %v", err)
	}
}

func TestTranslate_MissingTargetReturnsError(t *testing.T) {
	tr := NewTranslator(testWorkspaceRoot)
	raw := RawCommandRequest{
		ID:     "cmd-006",
		Intent: IntentBuild,
		// Target intentionally missing.
		Context: &CommandContext{
			WorkspaceRoot:   testWorkspaceRoot,
			RequestingAgent: "cli",
		},
	}

	_, err := tr.Translate(raw)
	if err == nil {
		t.Error("expected error for missing target, got nil")
	}
}

func TestTranslateFields_Convenience(t *testing.T) {
	tr := NewTranslator(testWorkspaceRoot)
	cmd, err := tr.TranslateFields("", IntentTest, "atlas", nil, "cli")
	if err != nil {
		t.Fatalf("TranslateFields error: %v", err)
	}
	if cmd.ID == "" {
		t.Error("ID should be generated")
	}
	if cmd.Context.RequestingAgent != "cli" {
		t.Errorf("RequestingAgent = %q, want cli", cmd.Context.RequestingAgent)
	}
}
