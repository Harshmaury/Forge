// @forge-project: forge
// @forge-path: internal/executor/intent/deploy.go
package intent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Harshmaury/Forge/internal/command"
)

// DeployHandler executes the "deploy" intent.
// Coordinates a deployment sequence via Nexus:
//   1. Build the project (reuses BuildHandler)
//   2. Instruct Nexus to stop the running service
//   3. Instruct Nexus to start the updated service
//
// Forge never manages service state directly.
type DeployHandler struct {
	nexusAddr   string
	httpClient  *http.Client
	buildHandler *BuildHandler
}

// NewDeployHandler creates a DeployHandler.
func NewDeployHandler(nexusAddr string) *DeployHandler {
	return &DeployHandler{
		nexusAddr:    nexusAddr,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		buildHandler: NewBuildHandler(),
	}
}

// Intent returns "deploy".
func (h *DeployHandler) Intent() string { return command.IntentDeploy }

// Execute runs build → stop → start via Nexus.
func (h *DeployHandler) Execute(ctx context.Context, cmd *command.Command) *Result {
	start := time.Now()
	result := &Result{
		CommandID: cmd.ID,
		Intent:    cmd.Intent,
		Target:    cmd.Target,
		Metadata:  map[string]string{},
	}

	// Step 1 — build.
	buildResult := h.buildHandler.Execute(ctx, cmd)
	if !buildResult.Success {
		result.Error = fmt.Sprintf("build failed: %s", buildResult.Error)
		result.Output = buildResult.Output
		result.Duration = time.Since(start)
		result.Metadata["stage"] = "build"
		return result
	}

	// Step 2 — stop via Nexus.
	if err := h.nexusProjectAction(ctx, cmd.Target, "stop"); err != nil {
		result.Error = fmt.Sprintf("nexus stop failed: %v", err)
		result.Duration = time.Since(start)
		result.Metadata["stage"] = "stop"
		return result
	}

	// Step 3 — start via Nexus.
	if err := h.nexusProjectAction(ctx, cmd.Target, "start"); err != nil {
		result.Error = fmt.Sprintf("nexus start failed: %v", err)
		result.Duration = time.Since(start)
		result.Metadata["stage"] = "start"
		return result
	}

	result.Duration = time.Since(start)
	result.Success = true
	result.Output = fmt.Sprintf("deployed %s: build ✓ → stop ✓ → start ✓", cmd.Target)
	result.Metadata["stage"] = "complete"
	return result
}

// nexusProjectAction sends a start or stop action to Nexus for a project.
func (h *DeployHandler) nexusProjectAction(ctx context.Context, target, action string) error {
	url := fmt.Sprintf("%s/projects/%s/%s", h.nexusAddr, target, action)
	body, _ := json.Marshal(map[string]string{})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("nexus unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("nexus %s returned HTTP %d", action, resp.StatusCode)
	}
	return nil
}
