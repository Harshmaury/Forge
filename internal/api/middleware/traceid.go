// @forge-project: forge
// @forge-path: internal/api/middleware/traceid.go
// TraceID middleware for Forge — Phase 4 / ADR-010.
// Mirrors Nexus and Atlas middleware/traceid.go exactly.
//
// Canon compliance: TraceIDHeader imported from identity — never from pkg/events.
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Harshmaury/Canon/identity"
)

type traceIDKey struct{}

// TraceID ensures every request carries an X-Trace-ID.
func TraceID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(identity.TraceIDHeader)
		if id == "" {
			id = fmt.Sprintf("forge-%d", time.Now().UnixNano())
		}
		ctx := context.WithValue(r.Context(), traceIDKey{}, id)
		w.Header().Set(identity.TraceIDHeader, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TraceIDFromContext extracts the trace ID from a context.
func TraceIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(traceIDKey{}).(string)
	return id
}
