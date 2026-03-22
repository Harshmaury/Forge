// @forge-project: Forge
// @forge-path: internal/api/handler/actor.go
// ActorFromRequest extracts the actor identity from an HTTP request.
// ADR-042: used by the execute handler to record who triggered a Forge command.
//
// Extraction is fail-open — if Gate is unreachable or no token is present,
// returns an empty ActorInfo. Guardian G-009 detects anonymous executions.
package handler

import (
	"net/http"

	"github.com/Harshmaury/Forge/internal/identity"
)

// ActorInfo holds the extracted identity for one request.
type ActorInfo struct {
	Subject string   // Gate sub — empty = anonymous
	Scopes  []string // Gate scp
}

// extractActor reads X-Identity-Token from r and validates via Gate.
// Returns empty ActorInfo (not an error) if absent or Gate is down.
func extractActor(extractor *identity.Extractor, r *http.Request) ActorInfo {
	if extractor == nil {
		return ActorInfo{}
	}
	actor := extractor.Extract(r.Context(), r)
	return ActorInfo{Subject: actor.Subject, Scopes: actor.Scopes}
}
