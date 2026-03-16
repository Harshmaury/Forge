// @forge-project: forge
// @forge-path: internal/api/handler/commands.go
// CommandHandler handles command submission and intent listing.
//
// Phase 4 (ADR-010):
//   - PreflightChecker runs before executor — verifies target in Atlas graph
//   - Every execution is logged to execution_history via store
//   - TraceID extracted from context and stored with each record
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/Harshmaury/Forge/internal/api/middleware"
	forgecontext "github.com/Harshmaury/Forge/internal/context"
	"github.com/Harshmaury/Forge/internal/command"
	"github.com/Harshmaury/Forge/internal/executor"
	"github.com/Harshmaury/Forge/internal/executor/intent"
	"github.com/Harshmaury/Forge/internal/preflight"
	"github.com/Harshmaury/Forge/internal/store"
)

// CommandHandler handles POST /commands.
type CommandHandler struct {
	translator *command.Translator
	resolver   *forgecontext.Resolver
	engine     *executor.Engine
	checker    *preflight.Checker // nil = preflight disabled
	store      store.Storer       // nil = history logging disabled
}

// NewCommandHandler creates a CommandHandler.
func NewCommandHandler(
	t *command.Translator,
	r *forgecontext.Resolver,
	e *executor.Engine,
	c *preflight.Checker,
	s store.Storer,
) *CommandHandler {
	return &CommandHandler{translator: t, resolver: r, engine: e, checker: c, store: s}
}

// Submit handles POST /commands.
// Phase 4: preflight check before execution, result logged to history.
func (h *CommandHandler) Submit(w http.ResponseWriter, r *http.Request) {
	var raw command.RawCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		respondErr(w, http.StatusBadRequest, fmt.Errorf("decode request: %w", err))
		return
	}

	// Translation layer — always before executor (ADR-004).
	cmd, err := h.translator.Translate(raw)
	if err != nil {
		respondErr(w, http.StatusBadRequest, err)
		return
	}

	// Context enrichment from Atlas + Nexus.
	cmd = h.resolver.ResolveContext(r.Context(), cmd)

	traceID := middleware.TraceIDFromContext(r.Context())
	startedAt := time.Now().UTC()

	// Phase 4: preflight — verify target is verified in Atlas graph (ADR-010).
	if h.checker != nil {
		pr := h.checker.Check(r.Context(), cmd.Target)
		if !pr.Permitted {
			h.recordDenied(cmd, traceID, startedAt, pr.Reason)
			respondErr(w, http.StatusUnprocessableEntity,
				fmt.Errorf("preflight denied: %s", pr.Reason))
			return
		}
	}

	// Execute.
	result := h.engine.Execute(r.Context(), cmd)
	finishedAt := time.Now().UTC()

	// Log to history (Phase 4).
	h.recordExecution(cmd, traceID, result, startedAt, finishedAt)

	if !result.Success {
		respondErr(w, http.StatusUnprocessableEntity,
			fmt.Errorf("%s", result.Error))
		return
	}

	respondOK(w, result.ToExecutionResult())
}

// recordExecution persists a completed execution — best effort, never panics.
func (h *CommandHandler) recordExecution(cmd *command.Command, traceID string, result *intent.Result, startedAt, finishedAt time.Time) {
	if h.store == nil {
		return
	}
	status := "success"
	if !result.Success {
		status = "failure"
	}
	_ = h.store.LogExecution(&store.ExecutionRecord{
		ID:         uuid.New().String(),
		CommandID:  cmd.ID,
		Intent:     cmd.Intent,
		Target:     cmd.Target,
		TraceID:    traceID,
		Status:     status,
		Output:     result.Output,
		Error:      result.Error,
		DurationMS: finishedAt.Sub(startedAt).Milliseconds(),
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
	})
}

// recordDenied persists a preflight-denied execution record.
func (h *CommandHandler) recordDenied(cmd *command.Command, traceID string, startedAt time.Time, reason string) {
	if h.store == nil {
		return
	}
	now := time.Now().UTC()
	_ = h.store.LogExecution(&store.ExecutionRecord{
		ID:         uuid.New().String(),
		CommandID:  cmd.ID,
		Intent:     cmd.Intent,
		Target:     cmd.Target,
		TraceID:    traceID,
		Status:     "denied",
		Error:      reason,
		DurationMS: now.Sub(startedAt).Milliseconds(),
		StartedAt:  startedAt,
		FinishedAt: now,
	})
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
