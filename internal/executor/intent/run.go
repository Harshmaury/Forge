// @forge-project: forge
// @forge-path: internal/executor/intent/run.go
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

// RunHandler executes the "run" intent.
// Delegates to Nexus — instructs Nexus to start the target project.
// Forge never starts services directly.
type RunHandler struct {
	nexusAddr  string
	httpClient *http.Client
}

// NewRunHandler creates a RunHandler.
func NewRunHandler(nexusAddr string) *RunHandler {
	return &RunHandler{
		nexusAddr:  nexusAddr,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// Intent returns "run".
func (h *RunHandler) Intent() string { return command.IntentRun }

// Execute instructs Nexus to start the target project.
func (h *RunHandler) Execute(ctx context.Context, cmd *command.Command) *Result {
	start := time.Now()
	result := &Result{
		CommandID: cmd.ID,
		Intent:    cmd.Intent,
		Target:    cmd.Target,
		Metadata:  map[string]string{"nexus_addr": h.nexusAddr},
	}

	// POST to Nexus: start the target project.
	url := fmt.Sprintf("%s/projects/%s/start", h.nexusAddr, cmd.Target)
	body, _ := json.Marshal(map[string]string{"command_id": cmd.ID})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		result.Error = fmt.Sprintf("build request: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("nexus unreachable: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	defer resp.Body.Close()

	result.Duration = time.Since(start)
	result.Metadata["http_status"] = fmt.Sprintf("%d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		result.Error = fmt.Sprintf("nexus returned HTTP %d for project start", resp.StatusCode)
		return result
	}

	result.Success = true
	result.Output = fmt.Sprintf("project %s start requested via Nexus", cmd.Target)
	return result
}
