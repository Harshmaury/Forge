// @forge-project: forge
// @forge-path: internal/trigger/model.go
// Package trigger handles automation trigger registration and event matching.
// A trigger maps one workspace event topic to one stored workflow (ADR-007).
package trigger

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Harshmaury/Forge/internal/store"
	nexusevents "github.com/Harshmaury/Nexus/pkg/events"
)

// ── SUPPORTED EVENTS ─────────────────────────────────────────────────────────

// SupportedEvents is the set of workspace event topics Forge can trigger on.
var SupportedEvents = map[string]bool{
	nexusevents.TopicWorkspaceFileCreated:     true,
	nexusevents.TopicWorkspaceFileModified:    true,
	nexusevents.TopicWorkspaceFileDeleted:     true,
	nexusevents.TopicWorkspaceUpdated:         true,
	nexusevents.TopicWorkspaceProjectDetected: true,
}

// ── API REQUEST TYPES ─────────────────────────────────────────────────────────

// CreateTriggerRequest is the HTTP body for POST /triggers.
type CreateTriggerRequest struct {
	Event      string `json:"event"`       // workspace topic
	WorkflowID string `json:"workflow_id"` // must exist in store
	Filter     Filter `json:"filter"`      // optional
}

// Filter scopes a trigger to specific files or projects.
// All supplied fields are AND-combined. Empty fields match everything.
type Filter struct {
	Extension string `json:"extension"` // e.g. ".go"
	Project   string `json:"project"`   // Atlas project ID
	Directory string `json:"directory"` // absolute path prefix
}

// Validate checks that a CreateTriggerRequest has all required fields.
func (r *CreateTriggerRequest) Validate() error {
	if r.Event == "" {
		return fmt.Errorf("event is required")
	}
	if !SupportedEvents[r.Event] {
		return fmt.Errorf("unsupported event %q — supported: workspace.file.created, .modified, .deleted, .updated, .project.detected", r.Event)
	}
	if r.WorkflowID == "" {
		return fmt.Errorf("workflow_id is required")
	}
	return nil
}

// ToStoreTrigger converts the request to a store.Trigger.
func (r *CreateTriggerRequest) ToStoreTrigger(id string) *store.Trigger {
	return &store.Trigger{
		ID:         id,
		Event:      r.Event,
		WorkflowID: r.WorkflowID,
		FilterExt:  r.Filter.Extension,
		FilterProj: r.Filter.Project,
		FilterDir:  r.Filter.Directory,
		Enabled:    true,
	}
}

// ── EVENT PAYLOAD ─────────────────────────────────────────────────────────────

// WorkspaceEventPayload carries the fields from a workspace event
// that are used for trigger filter matching.
type WorkspaceEventPayload struct {
	Path      string // absolute file path
	Extension string // file extension including dot
	Project   string // Atlas project ID (may be empty)
}

// ── FILTER MATCHING ───────────────────────────────────────────────────────────

// Matches returns true if the trigger's filter matches the given event payload.
// All non-empty filter fields must match (AND semantics).
func Matches(t *store.Trigger, payload WorkspaceEventPayload) bool {
	if t.FilterExt != "" {
		if !strings.EqualFold(payload.Extension, t.FilterExt) {
			return false
		}
	}
	if t.FilterProj != "" {
		if payload.Project != t.FilterProj {
			return false
		}
	}
	if t.FilterDir != "" {
		dir := filepath.Clean(t.FilterDir)
		filedir := filepath.Dir(filepath.Clean(payload.Path))
		if !strings.HasPrefix(filedir+string(filepath.Separator),
			dir+string(filepath.Separator)) && filedir != dir {
			return false
		}
	}
	return true
}
