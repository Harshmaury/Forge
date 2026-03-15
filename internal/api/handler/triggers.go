// @forge-project: forge
// @forge-path: internal/api/handler/triggers.go
// TriggerHandler handles trigger CRUD endpoints.
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Harshmaury/Forge/internal/store"
	"github.com/Harshmaury/Forge/internal/trigger"
	"github.com/google/uuid"
)

// TriggerHandler handles /triggers routes.
type TriggerHandler struct {
	store store.Storer
}

// NewTriggerHandler creates a TriggerHandler.
func NewTriggerHandler(s store.Storer) *TriggerHandler {
	return &TriggerHandler{store: s}
}

// Create handles POST /triggers
func (h *TriggerHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req trigger.CreateTriggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErr(w, http.StatusBadRequest, fmt.Errorf("decode request: %w", err))
		return
	}
	if err := req.Validate(); err != nil {
		respondErr(w, http.StatusBadRequest, err)
		return
	}

	// Verify workflow exists.
	wf, err := h.store.GetWorkflow(req.WorkflowID)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, fmt.Errorf("lookup workflow: %w", err))
		return
	}
	if wf == nil {
		respondErr(w, http.StatusBadRequest,
			fmt.Errorf("workflow %q not found", req.WorkflowID))
		return
	}

	t := req.ToStoreTrigger(uuid.New().String())
	if err := h.store.CreateTrigger(t); err != nil {
		respondErr(w, http.StatusInternalServerError, fmt.Errorf("create trigger: %w", err))
		return
	}

	respondOK(w, map[string]any{"trigger": t})
}

// List handles GET /triggers
func (h *TriggerHandler) List(w http.ResponseWriter, r *http.Request) {
	triggers, err := h.store.GetAllTriggers()
	if err != nil {
		respondErr(w, http.StatusInternalServerError, fmt.Errorf("list triggers: %w", err))
		return
	}
	respondOK(w, map[string]any{"triggers": triggers, "total": len(triggers)})
}

// Delete handles DELETE /triggers/:id
func (h *TriggerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondErr(w, http.StatusBadRequest, fmt.Errorf("trigger id required"))
		return
	}

	t, err := h.store.GetTrigger(id)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, fmt.Errorf("get trigger: %w", err))
		return
	}
	if t == nil {
		respondErr(w, http.StatusNotFound, fmt.Errorf("trigger %q not found", id))
		return
	}

	if err := h.store.DeleteTrigger(id); err != nil {
		respondErr(w, http.StatusInternalServerError, fmt.Errorf("delete trigger: %w", err))
		return
	}

	respondOK(w, map[string]any{"deleted": id})
}
