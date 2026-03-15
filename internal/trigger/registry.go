// @forge-project: forge
// @forge-path: internal/trigger/registry.go
// Registry loads enabled triggers from the store and returns those
// matching a given workspace event. Pure read layer — no execution.
package trigger

import (
	"fmt"

	"github.com/Harshmaury/Forge/internal/store"
)

// Registry evaluates workspace events against stored triggers.
type Registry struct {
	store store.Storer
}

// NewRegistry creates a Registry.
func NewRegistry(s store.Storer) *Registry {
	return &Registry{store: s}
}

// MatchingTriggers returns all enabled triggers for the given event topic
// whose filters match the payload. Returns an empty slice if none match.
func (r *Registry) MatchingTriggers(
	event string,
	payload WorkspaceEventPayload,
) ([]*store.Trigger, error) {
	triggers, err := r.store.GetEnabledTriggersByEvent(event)
	if err != nil {
		return nil, fmt.Errorf("load triggers for %s: %w", event, err)
	}

	var matched []*store.Trigger
	for _, t := range triggers {
		if Matches(t, payload) {
			matched = append(matched, t)
		}
	}
	return matched, nil
}
