// @forge-project: forge
// @forge-path: internal/api/handler/workflows.go
// FG-H-03: Create wraps workflow + step insertion in a transaction.
//   Previously if AddStep failed mid-way, the workflow row and partial
//   steps were committed — subsequent GET returned a broken workflow.
//   Now the entire create is atomic: all steps succeed or nothing is stored.
//
// FG-H-04: Run returns correct HTTP status codes.
//   Previously all executor errors mapped to 404. Store errors and step
//   failures now return 500 and 422 respectively; only missing workflow
//   returns 404.
//
// WorkflowHandler handles workflow CRUD and execution endpoints.
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	forgecontext "github.com/Harshmaury/Forge/internal/context"
	"github.com/Harshmaury/Forge/internal/command"
	"github.com/Harshmaury/Forge/internal/store"
	"github.com/Harshmaury/Forge/internal/workflow"
	"github.com/google/uuid"
)

// WorkflowHandler handles /workflows routes.
type WorkflowHandler struct {
	store    store.Storer
	executor *workflow.Executor
	resolver *forgecontext.Resolver
}

// NewWorkflowHandler creates a WorkflowHandler.
func NewWorkflowHandler(
	s store.Storer,
	e *workflow.Executor,
	r *forgecontext.Resolver,
) *WorkflowHandler {
	return &WorkflowHandler{store: s, executor: e, resolver: r}
}

// Create handles POST /workflows
// FG-H-03: workflow row + all steps created atomically via WithWorkflowTransaction.
func (h *WorkflowHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req workflow.CreateWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErr(w, http.StatusBadRequest, fmt.Errorf("decode request: %w", err))
		return
	}
	if err := req.Validate(); err != nil {
		respondErr(w, http.StatusBadRequest, err)
		return
	}

	wf := &store.Workflow{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Trigger:     "manual",
	}

	// Wrap creation + step insertion in a transaction so partial failures
	// never leave a broken workflow in the store (FG-H-03).
	if err := h.store.WithWorkflowTransaction(func() error {
		if err := h.store.CreateWorkflow(wf); err != nil {
			return fmt.Errorf("create workflow: %w", err)
		}
		for _, step := range req.ToStoreSteps(wf.ID) {
			if err := h.store.AddStep(step); err != nil {
				return fmt.Errorf("add step %d: %w", step.Position, err)
			}
		}
		return nil
	}); err != nil {
		respondErr(w, http.StatusInternalServerError, err)
		return
	}

	respondOK(w, map[string]any{"workflow": wf, "steps_added": len(req.Steps)})
}

// List handles GET /workflows
func (h *WorkflowHandler) List(w http.ResponseWriter, r *http.Request) {
	workflows, err := h.store.GetAllWorkflows()
	if err != nil {
		respondErr(w, http.StatusInternalServerError, fmt.Errorf("list workflows: %w", err))
		return
	}
	respondOK(w, map[string]any{"workflows": workflows, "total": len(workflows)})
}

// Get handles GET /workflows/:id
func (h *WorkflowHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondErr(w, http.StatusBadRequest, fmt.Errorf("workflow id required"))
		return
	}

	wf, err := h.store.GetWorkflow(id)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, fmt.Errorf("get workflow: %w", err))
		return
	}
	if wf == nil {
		respondErr(w, http.StatusNotFound, fmt.Errorf("workflow %q not found", id))
		return
	}

	steps, _ := h.store.GetSteps(id)
	respondOK(w, map[string]any{"workflow": wf, "steps": steps})
}

// Run handles POST /workflows/:id/run
func (h *WorkflowHandler) Run(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondErr(w, http.StatusBadRequest, fmt.Errorf("workflow id required"))
		return
	}

	// Build base context from request body or defaults.
	var body struct {
		Context *command.CommandContext `json:"context,omitempty"`
	}
	json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck — optional body

	baseCtx := command.CommandContext{
		RequestingAgent: "http",
		Timestamp:       time.Now().UTC(),
	}
	if body.Context != nil {
		baseCtx = *body.Context
		if baseCtx.RequestingAgent == "" {
			baseCtx.RequestingAgent = "http"
		}
		if baseCtx.Timestamp.IsZero() {
			baseCtx.Timestamp = time.Now().UTC()
		}
	}

	result, err := h.executor.Run(r.Context(), id, baseCtx)
	if err != nil {
		// FG-H-04: only "workflow not found" is a 404; store/other errors are 500.
		status := http.StatusInternalServerError
		if result == nil {
			// executor returns nil result only when workflow is not found.
			status = http.StatusNotFound
		}
		respondErr(w, status, err)
		return
	}

	status := http.StatusOK
	if !result.Success {
		status = http.StatusUnprocessableEntity
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(apiResponse{OK: result.Success, Data: result}) //nolint:errcheck
}
