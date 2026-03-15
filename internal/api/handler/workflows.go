// @forge-project: forge
// @forge-path: internal/api/handler/workflows.go
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

	if err := h.store.CreateWorkflow(wf); err != nil {
		respondErr(w, http.StatusInternalServerError, fmt.Errorf("create workflow: %w", err))
		return
	}

	for _, step := range req.ToStoreSteps(wf.ID) {
		if err := h.store.AddStep(step); err != nil {
			respondErr(w, http.StatusInternalServerError, fmt.Errorf("add step: %w", err))
			return
		}
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
		respondErr(w, http.StatusNotFound, err)
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
