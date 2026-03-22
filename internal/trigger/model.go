// @forge-project: forge
// @forge-path: internal/trigger/model.go
// FG-H-06: SupportedEvents map key type changed from string to
//   canonevents.Topic for consistency with FG-H-02 (subscriber lookup).
//   The map is populated from topic constants — no string literals.
//
// Package trigger handles automation trigger registration and event matching.
// A trigger maps one workspace event topic to one stored workflow (ADR-007).
package trigger

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Harshmaury/Forge/internal/store"
	canonevents "github.com/Harshmaury/Canon/events" // ADR-045: migrated from Nexus/pkg/events
)

// ── SUPPORTED EVENTS ─────────────────────────────────────────────────────────

// SupportedEvents is the set of workspace event topics Forge can trigger on.
// FG-H-06: keyed by canonevents.Topic (not string) for type-safe lookup.
var SupportedEvents = map[canonevents.TopicType]bool{
	canonevents.TopicWorkspaceFileCreated:     true,
	canonevents.TopicWorkspaceFileModified:    true,
	canonevents.TopicWorkspaceFileDeleted:     true,
	canonevents.TopicWorkspaceUpdated:         true,
	canonevents.TopicWorkspaceProjectDetected: true,
}

// ── API REQUEST TYPES ─────────────────────────────────────────────────────────

// CreateTriggerRequest is the HTTP body for POST /triggers.
type CreateTriggerRequest struct {
	Event      string `json:"event"`       // workspace topic (empty for cron triggers)
	WorkflowID string `json:"workflow_id"` // must exist in store
	Filter     Filter `json:"filter"`      // optional
	Schedule   string `json:"schedule"`    // @every <dur> | @hourly | @daily (empty = event trigger)
}

// Filter scopes a trigger to specific files or projects.
// All supplied fields are AND-combined. Empty fields match everything.
type Filter struct {
	Extension string `json:"extension"` // e.g. ".go"
	Project   string `json:"project"`   // Atlas project ID
	Directory string `json:"directory"` // absolute path prefix
}

// Validate checks that a CreateTriggerRequest has all required fields.
// A trigger must have either an event (event trigger) or a schedule (cron trigger), not neither.
func (r *CreateTriggerRequest) Validate() error {
	if r.WorkflowID == "" {
		return fmt.Errorf("workflow_id is required")
	}
	if r.Schedule != "" {
		// Cron trigger — event field is ignored.
		_, err := parseSchedule(r.Schedule)
		if err != nil {
			return fmt.Errorf("invalid schedule: %w", err)
		}
		return nil
	}
	// Event trigger — event field required.
	if r.Event == "" {
		return fmt.Errorf("either event or schedule is required")
	}
	if !SupportedEvents[canonevents.TopicType(r.Event)] {
		return fmt.Errorf("unsupported event %q — supported: workspace.file.created, .modified, .deleted, .updated, .project.detected", r.Event)
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
		Schedule:   r.Schedule,
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
