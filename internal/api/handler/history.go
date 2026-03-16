// @forge-project: forge
// @forge-path: internal/api/handler/history.go
// HistoryHandler handles execution history routes (Phase 4 / ADR-010).
//
// GET /history           — paginated list, most recent first
// GET /history/:trace_id — all executions for a trace ID
//
// These endpoints enable the developer to audit what ran and when,
// and provide the data surface for future Guardian observer service.
package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Harshmaury/Forge/internal/store"
)

const defaultHistoryLimit = 20

// HistoryHandler handles GET /history routes.
type HistoryHandler struct {
	store store.Storer
}

// NewHistoryHandler creates a HistoryHandler.
func NewHistoryHandler(s store.Storer) *HistoryHandler {
	return &HistoryHandler{store: s}
}

// List handles GET /history
// Returns recent execution records. Supports ?limit=N (default 20, max 200).
func (h *HistoryHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := defaultHistoryLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			respondErr(w, http.StatusBadRequest,
				fmt.Errorf("limit must be a positive integer, got %q", raw))
			return
		}
		if n > 200 {
			n = 200
		}
		limit = n
	}

	records, err := h.store.GetHistory(limit)
	if err != nil {
		respondErr(w, http.StatusInternalServerError,
			fmt.Errorf("get history: %w", err))
		return
	}
	respondOK(w, records)
}

// ByTrace handles GET /history/:trace_id
// Returns all execution records for a given trace ID in ascending order.
func (h *HistoryHandler) ByTrace(w http.ResponseWriter, r *http.Request) {
	traceID := r.PathValue("trace_id")
	if traceID == "" {
		respondErr(w, http.StatusBadRequest, fmt.Errorf("trace_id required"))
		return
	}

	records, err := h.store.GetHistoryByTrace(traceID)
	if err != nil {
		respondErr(w, http.StatusInternalServerError,
			fmt.Errorf("get history by trace %q: %w", traceID, err))
		return
	}
	respondOK(w, records)
}
