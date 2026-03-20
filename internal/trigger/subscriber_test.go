// @forge-project: forge
// @forge-path: internal/trigger/subscriber_test.go
package trigger

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Harshmaury/Forge/internal/command"
	forgecontext "github.com/Harshmaury/Forge/internal/context"
	"github.com/Harshmaury/Forge/internal/executor"
	"github.com/Harshmaury/Forge/internal/executor/intent"
	"github.com/Harshmaury/Forge/internal/store"
	"github.com/Harshmaury/Forge/internal/workflow"
	atlasclient "github.com/Harshmaury/Forge/internal/atlas"
	nexusclient "github.com/Harshmaury/Forge/internal/nexus"
	nexusevents "github.com/Harshmaury/Nexus/pkg/events"
)

// ── MOCK CLIENTS ─────────────────────────────────────────────────────────────

type mockNexus struct{}
type mockAtlas struct{}

func (m *mockNexus) GetProject(_ context.Context, id string) (*nexusclient.Project, error) {
	return &nexusclient.Project{ID: id}, nil
}
func (m *mockAtlas) GetProject(_ context.Context, id string) (*atlasclient.ProjectDetail, error) {
	return &atlasclient.ProjectDetail{ID: id, Language: "go", Path: "/workspace/" + id}, nil
}
func (m *mockAtlas) GetWorkspaceContext(_ context.Context) (*atlasclient.WorkspaceContext, error) {
	return &atlasclient.WorkspaceContext{WorkspaceRoot: "/home/harsh/workspace"}, nil
}

// ── MOCK WORKFLOW EXECUTOR ────────────────────────────────────────────────────

type mockWorkflowStore struct {
	mockStore
	workflow *store.Workflow
	steps    []*store.WorkflowStep
}

func (m *mockWorkflowStore) GetWorkflow(id string) (*store.Workflow, error) {
	return m.workflow, nil
}
func (m *mockWorkflowStore) GetSteps(id string) ([]*store.WorkflowStep, error) {
	return m.steps, nil
}

type captureHandler struct {
	mu       sync.Mutex
	executed []string
}

func (h *captureHandler) Intent() string { return command.IntentBuild }
func (h *captureHandler) Execute(_ context.Context, cmd *command.Command) *intent.Result {
	h.mu.Lock()
	h.executed = append(h.executed, cmd.Target)
	h.mu.Unlock()
	return &intent.Result{CommandID: cmd.ID, Intent: cmd.Intent, Target: cmd.Target, Success: true}
}

// ── PAYLOAD EXTRACTION ────────────────────────────────────────────────────────

func TestExtractPayload_FileModified(t *testing.T) {
	s := &Subscriber{logger: log.New(os.Stderr, "[test] ", 0)}

	p := nexusevents.WorkspaceFilePayload{
		Path:      "/workspace/nexus/main.go",
		Extension: ".go",
		SizeBytes: 1024,
		EventAt:   time.Now(),
	}
	raw, _ := json.Marshal(p)

	got := s.extractPayload(nexusevents.TopicWorkspaceFileModified, raw)
	if got.Path != "/workspace/nexus/main.go" {
		t.Errorf("Path = %q, want /workspace/nexus/main.go", got.Path)
	}
	if got.Extension != ".go" {
		t.Errorf("Extension = %q, want .go", got.Extension)
	}
}

func TestExtractPayload_WorkspaceUpdated(t *testing.T) {
	s := &Subscriber{logger: log.New(os.Stderr, "[test] ", 0)}
	// workspace.updated has a different payload — returns empty, not an error
	raw := json.RawMessage(`{"watch_dir":"/workspace","event_at":"2026-03-15T00:00:00Z"}`)
	got := s.extractPayload(nexusevents.TopicWorkspaceUpdated, raw)
	// Empty payload — trigger with no filter still fires
	if got.Path != "" {
		t.Errorf("expected empty path for workspace.updated, got %q", got.Path)
	}
}

func TestExtractPayload_InvalidJSON(t *testing.T) {
	s := &Subscriber{logger: log.New(os.Stderr, "[test] ", 0)}
	got := s.extractPayload(nexusevents.TopicWorkspaceFileModified, json.RawMessage(`not json`))
	// Should return empty payload, not panic
	if got.Path != "" {
		t.Errorf("expected empty payload for invalid JSON, got path=%q", got.Path)
	}
}

// ── DISPATCH ─────────────────────────────────────────────────────────────────

func TestDispatch_FiresMatchingWorkflow(t *testing.T) {
	capture := &captureHandler{}

	eng := executor.NewEngine()
	eng.Register(capture)

	wfStore := &mockWorkflowStore{
		workflow: &store.Workflow{ID: "wf-001", Name: "test", Trigger: "event"},
		steps: []*store.WorkflowStep{
			{WorkflowID: "wf-001", Position: 1, Intent: command.IntentBuild,
				Target: "nexus", Parameters: map[string]string{}},
		},
	}

	resolver := forgecontext.NewResolver(
		&mockNexus{}, &mockAtlas{},
		log.New(os.Stderr, "[test] ", 0),
	)
	wfExecutor := workflow.NewExecutor(wfStore, eng, resolver, log.New(os.Stderr, "[test] ", 0))

	triggerStore := &mockStore{triggers: []*store.Trigger{
		{ID: "t1", Event: nexusevents.TopicWorkspaceFileModified,
			WorkflowID: "wf-001", FilterExt: ".go", Enabled: true},
	}}
	registry := NewRegistry(triggerStore)

	s := &Subscriber{
		nexusAddr:  "http://127.0.0.1:8080",
		registry:   registry,
		executor:   wfExecutor,
		logger:     log.New(os.Stderr, "[test] ", 0),
		httpClient: nil, // not used in dispatch test
		sem:        make(chan struct{}, 8),
	}

	p := WorkspaceEventPayload{Path: "/workspace/nexus/main.go", Extension: ".go"}
	s.dispatch(context.Background(), nexusevents.TopicWorkspaceFileModified, p)

	// Give goroutine time to complete.
	time.Sleep(100 * time.Millisecond)

	capture.mu.Lock()
	defer capture.mu.Unlock()
	if len(capture.executed) != 1 {
		t.Errorf("expected 1 execution, got %d", len(capture.executed))
	}
}

func TestDispatch_NoMatchNoExecution(t *testing.T) {
	capture := &captureHandler{}
	eng := executor.NewEngine()
	eng.Register(capture)

	triggerStore := &mockStore{triggers: []*store.Trigger{
		{ID: "t1", Event: nexusevents.TopicWorkspaceFileModified,
			WorkflowID: "wf-001", FilterExt: ".go", Enabled: true},
	}}
	registry := NewRegistry(triggerStore)

	s := &Subscriber{
		registry: registry,
		executor: nil, // not called if no match
		logger:   log.New(os.Stderr, "[test] ", 0),
	}

	// .md file — should not match .go filter
	p := WorkspaceEventPayload{Path: "/workspace/nexus/README.md", Extension: ".md"}
	s.dispatch(context.Background(), nexusevents.TopicWorkspaceFileModified, p)

	time.Sleep(50 * time.Millisecond)

	capture.mu.Lock()
	defer capture.mu.Unlock()
	if len(capture.executed) != 0 {
		t.Errorf("expected 0 executions for non-matching event, got %d", len(capture.executed))
	}
}
