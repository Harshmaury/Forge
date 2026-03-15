// @forge-project: forge
// @forge-path: internal/executor/intent/build.go
package intent

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/Harshmaury/Forge/internal/command"
)

// BuildHandler executes the "build" intent.
// Runs the language-appropriate build command in the project directory.
type BuildHandler struct{}

// NewBuildHandler creates a BuildHandler.
func NewBuildHandler() *BuildHandler { return &BuildHandler{} }

// Intent returns "build".
func (h *BuildHandler) Intent() string { return command.IntentBuild }

// Execute runs the build command for the target project.
func (h *BuildHandler) Execute(ctx context.Context, cmd *command.Command) *Result {
	start := time.Now()
	result := &Result{
		CommandID: cmd.ID,
		Intent:    cmd.Intent,
		Target:    cmd.Target,
		Metadata:  map[string]string{},
	}

	// Determine build command from project language.
	lang := cmd.Context.Language
	buildCmd := BuildCmdForLanguage(lang)
	if len(buildCmd) == 0 {
		result.Error = fmt.Sprintf("no build command known for language %q", lang)
		result.Duration = time.Since(start)
		return result
	}

	// Determine working directory.
	dir := cmd.Context.ProjectPath
	if dir == "" {
		result.Error = "project path not available in command context"
		result.Duration = time.Since(start)
		return result
	}

	// Allow caller to pass extra args via parameters.
	args := append(buildCmd[1:], cmd.Parameters["args"])
	if cmd.Parameters["args"] == "" {
		args = buildCmd[1:]
	}

	//nolint:gosec — command and args are from a trusted allowlist
	c := exec.CommandContext(ctx, buildCmd[0], args...)
	c.Dir = dir

	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr

	err := c.Run()
	result.Duration = time.Since(start)
	result.Output = stdout.String()
	result.Metadata["command"] = fmt.Sprintf("%v", buildCmd)
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
