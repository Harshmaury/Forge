// @forge-project: forge
// @forge-path: internal/executor/intent/build_test.go
package intent

import (
	"context"
	"testing"
	"time"

	"github.com/Harshmaury/Forge/internal/command"
)

func buildCmd(lang, dir string) *command.Command {
	return &command.Command{
		ID:         "test-build-001",
		Intent:     command.IntentBuild,
		Target:     "nexus",
		Parameters: map[string]string{},
		Context: command.CommandContext{
			WorkspaceRoot:   "/home/harsh/workspace",
			ProjectPath:     dir,
			Language:        lang,
			RequestingAgent: "test",
			Timestamp:       time.Now(),
		},
	}
}

// ── Handler interface ─────────────────────────────────────────────────────────

func TestBuildHandler_Intent(t *testing.T) {
	h := NewBuildHandler()
	if h.Intent() != command.IntentBuild {
		t.Errorf("Intent() = %q, want %q", h.Intent(), command.IntentBuild)
	}
}

func TestTestHandler_Intent(t *testing.T) {
	h := NewTestHandler()
	if h.Intent() != command.IntentTest {
		t.Errorf("Intent() = %q, want %q", h.Intent(), command.IntentTest)
	}
}

func TestRunHandler_Intent(t *testing.T) {
	h := NewRunHandler("http://127.0.0.1:8080")
	if h.Intent() != command.IntentRun {
		t.Errorf("Intent() = %q, want %q", h.Intent(), command.IntentRun)
	}
}

func TestDeployHandler_Intent(t *testing.T) {
	h := NewDeployHandler("http://127.0.0.1:8080")
	if h.Intent() != command.IntentDeploy {
		t.Errorf("Intent() = %q, want %q", h.Intent(), command.IntentDeploy)
	}
}

// ── Language command lookup ───────────────────────────────────────────────────

func TestBuildCmdForLanguage(t *testing.T) {
	cases := []struct {
		lang    string
		wantNil bool
	}{
		{"go", false},
		{"python", false},
		{"node", false},
		{"rust", false},
		{"dotnet", false},
		{"unknown", true},
		{"", true},
	}
	for _, tc := range cases {
		cmd := BuildCmdForLanguage(tc.lang)
		if tc.wantNil && cmd != nil {
			t.Errorf("BuildCmdForLanguage(%q) = %v, want nil", tc.lang, cmd)
		}
		if !tc.wantNil && cmd == nil {
			t.Errorf("BuildCmdForLanguage(%q) = nil, want non-nil", tc.lang)
		}
	}
}

func TestTestCmdForLanguage(t *testing.T) {
	if TestCmdForLanguage("go") == nil {
		t.Error("TestCmdForLanguage(go) should not be nil")
	}
	if TestCmdForLanguage("unknown") != nil {
		t.Error("TestCmdForLanguage(unknown) should be nil")
	}
}

// ── Build error cases ─────────────────────────────────────────────────────────

func TestBuildHandler_UnknownLanguage(t *testing.T) {
	h := NewBuildHandler()
	cmd := buildCmd("cobol", "/some/dir")
	result := h.Execute(context.Background(), cmd)

	if result.Success {
		t.Error("expected failure for unknown language")
	}
	if result.Error == "" {
		t.Error("expected error message for unknown language")
	}
}

func TestBuildHandler_EmptyProjectPath(t *testing.T) {
	h := NewBuildHandler()
	cmd := buildCmd("go", "") // empty path

	result := h.Execute(context.Background(), cmd)
	if result.Success {
		t.Error("expected failure for empty project path")
	}
	if result.Error == "" {
		t.Error("expected error message for empty project path")
	}
}

// ── Result.ToExecutionResult ─────────────────────────────────────────────────

func TestResult_ToExecutionResult(t *testing.T) {
	r := &Result{
		CommandID: "cmd-001",
		Intent:    command.IntentBuild,
		Target:    "nexus",
		Success:   true,
		Output:    "build ok",
		Duration:  100 * time.Millisecond,
	}

	er := r.ToExecutionResult()
	if er.CommandID != "cmd-001" {
		t.Errorf("CommandID = %q, want cmd-001", er.CommandID)
	}
	if !er.Success {
		t.Error("Success should be true")
	}
	if er.Duration == "" {
		t.Error("Duration should not be empty")
	}
}
