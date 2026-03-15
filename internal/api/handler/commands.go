// @forge-project: forge
// @forge-path: internal/api/handler/commands.go
// CommandHandler handles command submission and intent listing.
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	forgecontext "github.com/Harshmaury/Forge/internal/context"
	"github.com/Harshmaury/Forge/internal/command"
	"github.com/Harshmaury/Forge/internal/executor"
)

// CommandHandler handles POST /commands.
type CommandHandler struct {
	translator *command.Translator
	resolver   *forgecontext.Resolver
	engine     *executor.Engine
}

// NewCommandHandler creates a CommandHandler.
func NewCommandHandler(
	t *command.Translator,
	r *forgecontext.Resolver,
	e *executor.Engine,
) *CommandHandler {
	return &CommandHandler{translator: t, resolver: r, engine: e}
}

// Submit handles POST /commands
// Accepts a RawCommandRequest, translates, enriches, executes, returns result.
func (h *CommandHandler) Submit(w http.ResponseWriter, r *http.Request) {
	var raw command.RawCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		respondErr(w, http.StatusBadRequest, fmt.Errorf("decode request: %w", err))
		return
	}

	// Translation layer — always before executor.
	cmd, err := h.translator.Translate(raw)
	if err != nil {
		respondErr(w, http.StatusBadRequest, err)
		return
	}

	// Context enrichment from Atlas + Nexus.
	cmd = h.resolver.ResolveContext(r.Context(), cmd)

	// Execute.
	result := h.engine.Execute(r.Context(), cmd)

	if !result.Success {
		respondErr(w, http.StatusUnprocessableEntity,
			fmt.Errorf("%s", result.Error))
		return
	}

	respondOK(w, result.ToExecutionResult())
}

// IntentsHandler handles GET /intents.
type IntentsHandler struct {
	engine *executor.Engine
}

// NewIntentsHandler creates an IntentsHandler.
func NewIntentsHandler(e *executor.Engine) *IntentsHandler {
	return &IntentsHandler{engine: e}
}

// List handles GET /intents — returns all registered intent names.
func (h *IntentsHandler) List(w http.ResponseWriter, r *http.Request) {
	respondOK(w, map[string]any{
		"intents": h.engine.RegisteredIntents(),
	})
}

// ── RESPONSE HELPERS ──────────────────────────────────────────────────────────

type apiResponse struct {
	OK    bool   `json:"ok"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

func respondOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiResponse{OK: true, Data: data}) //nolint:errcheck
}

func respondErr(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(apiResponse{OK: false, Error: err.Error()}) //nolint:errcheck
}
