// @forge-project: forge
// @forge-path: internal/executor/intent/test.go
package intent

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/Harshmaury/Forge/internal/command"
)

// TestHandler executes the "test" intent.
// Runs the language-appropriate test command in the project directory.
type TestHandler struct{}

// NewTestHandler creates a TestHandler.
func NewTestHandler() *TestHandler { return &TestHandler{} }

// Intent returns "test".
func (h *TestHandler) Intent() string { return command.IntentTest }

// Execute runs the test suite for the target project.
func (h *TestHandler) Execute(ctx context.Context, cmd *command.Command) *Result {
	start := time.Now()
	result := &Result{
		CommandID: cmd.ID,
		Intent:    cmd.Intent,
		Target:    cmd.Target,
		Metadata:  map[string]string{},
	}

	lang := cmd.Context.Language
	testCmd := TestCmdForLanguage(lang)
	if len(testCmd) == 0 {
		result.Error = fmt.Sprintf("no test command known for language %q", lang)
		result.Duration = time.Since(start)
		return result
	}

	dir := cmd.Context.ProjectPath
	if dir == "" {
		result.Error = "project path not available in command context"
		result.Duration = time.Since(start)
		return result
	}

	args := testCmd[1:]
	if extra := cmd.Parameters["args"]; extra != "" {
		args = append(args, extra)
	}

	//nolint:gosec — command and args are from a trusted allowlist
	c := exec.CommandContext(ctx, testCmd[0], args...)
	c.Dir = dir

	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr

	err := c.Run()
	result.Duration = time.Since(start)
	result.Output = stdout.String()
	result.Metadata["command"] = fmt.Sprintf("%v", testCmd)
	result.Metadata["language"] = lang
	result.Metadata["dir"] = dir

	if err != nil {
		result.Error = stderr.String()
		if result.Error == "" {
			result.Error = err.Error()
		}
		return result
	}

	result.Success = true
	return result
}
