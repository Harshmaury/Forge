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
	"log"
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
	logger     *log.Logger
}

// NewCommandHandler creates a CommandHandler.
func NewCommandHandler(
	t *command.Translator,
	r *forgecontext.Resolver,
	e *executor.Engine,
	c *preflight.Checker,
	s store.Storer,
	l *log.Logger,
) *CommandHandler {
	if l == nil {
		l = log.Default()
	}
	return &CommandHandler{translator: t, resolver: r, engine: e, checker: c, store: s, logger: l}
}

// Submit handles POST /commands.
// Phase 4: preflight check before execution, result logged to history.
// ADR-021: snapshot captured at check time — frozen before Execute() runs.
func (h *CommandHandler) Submit(w http.ResponseWriter, r *http.Request) {
	var raw command.RawCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		respondErr(w, http.StatusBadRequest, fmt.Errorf("decode request: %w", err))
		return
	}

	cmd, err := h.translator.Translate(raw)
	if err != nil {
		respondErr(w, http.StatusBadRequest, err)
		return
	}

	cmd       = h.resolver.ResolveContext(r.Context(), cmd)
	traceID   := middleware.TraceIDFromContext(r.Context())
	startedAt := time.Now().UTC()

	if h.checker != nil {
		h.submitWithPreflight(w, r, cmd, traceID, startedAt)
		return
	}

	result     := h.engine.Execute(r.Context(), cmd)
	finishedAt := time.Now().UTC()
	h.recordExecution(cmd, traceID, result, startedAt, finishedAt, store.PreflightSnapshot{})
	if !result.Success {
		respondErr(w, http.StatusUnprocessableEntity, fmt.Errorf("%s", result.Error))
		return
	}
	respondOK(w, result.ToExecutionResult())
}

// submitWithPreflight runs the preflight check, captures the snapshot,
// then executes — keeping the snapshot frozen across the Execute() call.
func (h *CommandHandler) submitWithPreflight(
	w http.ResponseWriter,
	r *http.Request,
	cmd *command.Command,
	traceID string,
	startedAt time.Time,
) {
	pr := h.checker.Check(r.Context(), cmd.Target)
	snap := store.PreflightSnapshot{
		AtlasQueried:  pr.Snapshot.AtlasQueried,
		ProjectFound:  pr.Snapshot.ProjectFound,
		ProjectID:     pr.Snapshot.ProjectID,
		ProjectStatus: pr.Snapshot.ProjectStatus,
		Capabilities:  pr.Snapshot.Capabilities,
		DependsOn:     pr.Snapshot.DependsOn,
		SnapshotAt:    pr.Snapshot.SnapshotAt.Format(time.RFC3339Nano),
	}
	if !pr.Permitted {
		h.recordDenied(cmd, traceID, startedAt, pr.Reason, snap)
		respondErr(w, http.StatusUnprocessableEntity,
			fmt.Errorf("preflight denied: %s", pr.Reason))
		return
	}
	result     := h.engine.Execute(r.Context(), cmd)
	finishedAt := time.Now().UTC()
	h.recordExecution(cmd, traceID, result, startedAt, finishedAt, snap)
	if !result.Success {
		respondErr(w, http.StatusUnprocessableEntity, fmt.Errorf("%s", result.Error))
		return
	}
	respondOK(w, result.ToExecutionResult())
}

// recordExecution persists a completed execution — best effort, never panics.
func (h *CommandHandler) recordExecution(
	cmd *command.Command,
	traceID string,
	result *intent.Result,
	startedAt, finishedAt time.Time,
	snap store.PreflightSnapshot,
) {
	if h.store == nil {
		return
	}
	status := "success"
	if !result.Success {
		status = "failure"
	}
	if err := h.store.LogExecution(&store.ExecutionRecord{
		ID:                uuid.New().String(),
		CommandID:         cmd.ID,
		Intent:            cmd.Intent,
		Target:            cmd.Target,
		TraceID:           traceID,
		Status:            status,
		Output:            result.Output,
		Error:             result.Error,
		DurationMS:        finishedAt.Sub(startedAt).Milliseconds(),
		StartedAt:         startedAt,
		FinishedAt:        finishedAt,
		PreflightSnapshot: snap,
	}); err != nil {
		h.logger.Printf("WARNING: record execution: trace=%s target=%s: %v", traceID, cmd.Target, err)
	}
}

// recordDenied persists a preflight-denied execution record.
func (h *CommandHandler) recordDenied(
	cmd *command.Command,
	traceID string,
	startedAt time.Time,
	reason string,
	snap store.PreflightSnapshot,
) {
	if h.store == nil {
		return
	}
	now := time.Now().UTC()
	if err := h.store.LogExecution(&store.ExecutionRecord{
		ID:                uuid.New().String(),
		CommandID:         cmd.ID,
		Intent:            cmd.Intent,
		Target:            cmd.Target,
		TraceID:           traceID,
		Status:            "denied",
		Error:             reason,
		DurationMS:        now.Sub(startedAt).Milliseconds(),
		StartedAt:         startedAt,
		FinishedAt:        now,
		PreflightSnapshot: snap,
	}); err != nil {
		h.logger.Printf("WARNING: record denied execution: trace=%s target=%s: %v", traceID, cmd.Target, err)
	}
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
