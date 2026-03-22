// @forge-project: forge
// @forge-path: internal/trigger/registry_test.go
package trigger

import (
	"testing"
	"time"

	"github.com/Harshmaury/Forge/internal/store"
	nexusevents "github.com/Harshmaury/Nexus/pkg/events"
)

// ── MOCK STORE ────────────────────────────────────────────────────────────────

type mockStore struct {
	triggers []*store.Trigger
	err      error
}

func (m *mockStore) GetEnabledTriggersByEvent(event string) ([]*store.Trigger, error) {
	if m.err != nil {
		return nil, m.err
	}
	var result []*store.Trigger
	for _, t := range m.triggers {
		if t.Event == event && t.Enabled {
			result = append(result, t)
		}
	}
	return result, nil
}

// Unused Storer methods.
func (m *mockStore) Close() error                                      { return nil }
func (m *mockStore) CreateWorkflow(w *store.Workflow) error            { return nil }
func (m *mockStore) GetWorkflow(id string) (*store.Workflow, error)    { return nil, nil }
func (m *mockStore) GetAllWorkflows() ([]*store.Workflow, error)       { return nil, nil }
func (m *mockStore) DeleteWorkflow(id string) error                    { return nil }
func (m *mockStore) AddStep(s *store.WorkflowStep) error               { return nil }
func (m *mockStore) GetSteps(id string) ([]*store.WorkflowStep, error) { return nil, nil }
func (m *mockStore) DeleteSteps(id string) error                       { return nil }
func (m *mockStore) CreateTrigger(t *store.Trigger) error              { return nil }
func (m *mockStore) GetTrigger(id string) (*store.Trigger, error)      { return nil, nil }
func (m *mockStore) GetAllTriggers() ([]*store.Trigger, error)         { return m.triggers, nil }
func (m *mockStore) LogExecution(r *store.ExecutionRecord) error                    { return nil }
func (m *mockStore) GetHistory(limit int) ([]*store.ExecutionRecord, error)         { return nil, nil }
func (m *mockStore) GetHistoryByTrace(id string) ([]*store.ExecutionRecord, error)  { return nil, nil }
func (m *mockStore) WithWorkflowTransaction(fn func() error) error                  { return fn() }
func (m *mockStore) DeleteTrigger(id string) error                     { return nil }
func (m *mockStore) GetEnabledCronTriggers() ([]*store.Trigger, error)  { return nil, nil }
func (m *mockStore) GetDedupRecord(commandID string) (*store.DedupRecord, error) { return nil, nil }
func (m *mockStore) SetDedupRecord(r *store.DedupRecord) error                   { return nil }


// ── FILTER MATCHING ───────────────────────────────────────────────────────────

func trigger(ext, proj, dir string) *store.Trigger {
	return &store.Trigger{
		ID: "t-001", Event: nexusevents.TopicWorkspaceFileModified,
		WorkflowID: "wf-001", FilterExt: ext, FilterProj: proj,
		FilterDir: dir, Enabled: true, CreatedAt: time.Now(),
	}
}

func payload(path, ext, proj string) WorkspaceEventPayload {
	return WorkspaceEventPayload{Path: path, Extension: ext, Project: proj}
}

func TestMatches_NoFilter(t *testing.T) {
	tr := trigger("", "", "")
	p := payload("/workspace/nexus/main.go", ".go", "nexus")
	if !Matches(tr, p) {
		t.Error("empty filter should match everything")
	}
}

func TestMatches_ExtensionFilter(t *testing.T) {
	tr := trigger(".go", "", "")
	if !Matches(tr, payload("/a/b.go", ".go", "nexus")) {
		t.Error("should match .go file")
	}
	if Matches(tr, payload("/a/b.md", ".md", "nexus")) {
		t.Error("should not match .md file")
	}
}

func TestMatches_ExtensionCaseInsensitive(t *testing.T) {
	tr := trigger(".GO", "", "")
	if !Matches(tr, payload("/a/b.go", ".go", "nexus")) {
		t.Error("extension match should be case-insensitive")
	}
}

func TestMatches_ProjectFilter(t *testing.T) {
	tr := trigger("", "nexus", "")
	if !Matches(tr, payload("/a/b.go", ".go", "nexus")) {
		t.Error("should match nexus project")
	}
	if Matches(tr, payload("/a/b.go", ".go", "atlas")) {
		t.Error("should not match atlas project")
	}
}

func TestMatches_DirectoryFilter(t *testing.T) {
	tr := trigger("", "", "/workspace/nexus")
	if !Matches(tr, payload("/workspace/nexus/cmd/main.go", ".go", "")) {
		t.Error("should match file inside directory")
	}
	if Matches(tr, payload("/workspace/atlas/main.go", ".go", "")) {
		t.Error("should not match file outside directory")
	}
}

func TestMatches_AllFiltersAND(t *testing.T) {
	tr := trigger(".go", "nexus", "/workspace/nexus")
	// All three match.
	if !Matches(tr, payload("/workspace/nexus/main.go", ".go", "nexus")) {
		t.Error("all filters match — should return true")
	}
	// Extension doesn't match.
	if Matches(tr, payload("/workspace/nexus/README.md", ".md", "nexus")) {
		t.Error("extension mismatch — should return false")
	}
	// Project doesn't match.
	if Matches(tr, payload("/workspace/nexus/main.go", ".go", "atlas")) {
		t.Error("project mismatch — should return false")
	}
}

// ── REGISTRY ─────────────────────────────────────────────────────────────────

func TestRegistry_MatchingTriggers_ReturnsMatches(t *testing.T) {
	s := &mockStore{triggers: []*store.Trigger{
		{ID: "t1", Event: nexusevents.TopicWorkspaceFileModified,
			WorkflowID: "wf1", FilterExt: ".go", Enabled: true},
		{ID: "t2", Event: nexusevents.TopicWorkspaceFileModified,
			WorkflowID: "wf2", FilterExt: ".md", Enabled: true},
		{ID: "t3", Event: nexusevents.TopicWorkspaceFileCreated,
			WorkflowID: "wf3", Enabled: true}, // different event
	}}

	r := NewRegistry(s)
	matches, err := r.MatchingTriggers(
		nexusevents.TopicWorkspaceFileModified,
		payload("/workspace/nexus/main.go", ".go", "nexus"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 1 || matches[0].ID != "t1" {
		t.Errorf("expected [t1], got %v", matches)
	}
}

func TestRegistry_MatchingTriggers_DisabledSkipped(t *testing.T) {
	s := &mockStore{triggers: []*store.Trigger{
		{ID: "t1", Event: nexusevents.TopicWorkspaceFileModified,
			WorkflowID: "wf1", Enabled: false}, // disabled
	}}

	r := NewRegistry(s)
	matches, err := r.MatchingTriggers(
		nexusevents.TopicWorkspaceFileModified,
		payload("/a/b.go", ".go", ""),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("disabled trigger should not match, got %d", len(matches))
	}
}

func TestRegistry_MatchingTriggers_EmptyWhenNoMatch(t *testing.T) {
	s := &mockStore{triggers: []*store.Trigger{}}
	r := NewRegistry(s)
	matches, err := r.MatchingTriggers(nexusevents.TopicWorkspaceFileModified,
		payload("/a/b.go", ".go", ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected empty, got %d", len(matches))
	}
}

// ── VALIDATE ─────────────────────────────────────────────────────────────────

func TestCreateTriggerRequest_Validate(t *testing.T) {
	cases := []struct {
		name    string
		req     CreateTriggerRequest
		wantErr bool
	}{
		{"valid", CreateTriggerRequest{
			Event: nexusevents.TopicWorkspaceFileModified, WorkflowID: "wf-001",
		}, false},
		{"missing event", CreateTriggerRequest{WorkflowID: "wf-001"}, true},
		{"unsupported event", CreateTriggerRequest{Event: "custom.event", WorkflowID: "wf-001"}, true},
		{"missing workflow_id", CreateTriggerRequest{Event: nexusevents.TopicWorkspaceFileModified}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			if tc.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
