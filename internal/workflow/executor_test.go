// @forge-project: forge
// @forge-path: internal/workflow/executor_test.go
package workflow

import (
	"context"
	"errors"
	"log"
	"os"
	"testing"
	"time"

	"github.com/Harshmaury/Forge/internal/command"
	forgecontext "github.com/Harshmaury/Forge/internal/context"
	"github.com/Harshmaury/Forge/internal/executor"
	"github.com/Harshmaury/Forge/internal/executor/intent"
	"github.com/Harshmaury/Forge/internal/store"

	atlasclient "github.com/Harshmaury/Forge/internal/atlas"
	nexusclient "github.com/Harshmaury/Forge/internal/nexus"
)

// ── MOCK STORE ────────────────────────────────────────────────────────────────

type mockStore struct {
	workflows map[string]*store.Workflow
	steps     map[string][]*store.WorkflowStep
	err       error
}

func newMockStore() *mockStore {
	return &mockStore{
		workflows: make(map[string]*store.Workflow),
		steps:     make(map[string][]*store.WorkflowStep),
	}
}

func (m *mockStore) CreateWorkflow(w *store.Workflow) error {
	if m.err != nil {
		return m.err
	}
	m.workflows[w.ID] = w
	return nil
}
func (m *mockStore) GetWorkflow(id string) (*store.Workflow, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.workflows[id], nil
}
func (m *mockStore) GetAllWorkflows() ([]*store.Workflow, error) {
	wfs := make([]*store.Workflow, 0, len(m.workflows))
	for _, w := range m.workflows {
		wfs = append(wfs, w)
	}
	return wfs, m.err
}
func (m *mockStore) DeleteWorkflow(id string) error {
	delete(m.workflows, id)
	delete(m.steps, id)
	return m.err
}
func (m *mockStore) AddStep(s *store.WorkflowStep) error {
	if m.err != nil {
		return m.err
	}
	m.steps[s.WorkflowID] = append(m.steps[s.WorkflowID], s)
	return nil
}
func (m *mockStore) GetSteps(workflowID string) ([]*store.WorkflowStep, error) {
	return m.steps[workflowID], m.err
}
func (m *mockStore) DeleteSteps(workflowID string) error {
	delete(m.steps, workflowID)
	return m.err
}
func (m *mockStore) Close() error { return nil }

// Trigger stubs — satisfy store.Storer (Phase 3 methods not used by workflow executor tests).
func (m *mockStore) CreateTrigger(t *store.Trigger) error                        { return nil }
func (m *mockStore) GetTrigger(id string) (*store.Trigger, error)                { return nil, nil }
func (m *mockStore) GetAllTriggers() ([]*store.Trigger, error)                   { return nil, nil }
func (m *mockStore) GetEnabledTriggersByEvent(event string) ([]*store.Trigger, error) { return nil, nil }
func (m *mockStore) GetEnabledCronTriggers() ([]*store.Trigger, error)           { return nil, nil }
func (m *mockStore) DeleteTrigger(id string) error                               { return nil }

// Transaction stub — satisfy store.Storer.
func (m *mockStore) WithWorkflowTransaction(fn func() error) error           { return fn() }

// Execution history stubs — Phase 4 methods not used by workflow executor tests.
func (m *mockStore) LogExecution(r *store.ExecutionRecord) error                        { return nil }
func (m *mockStore) GetHistory(limit int) ([]*store.ExecutionRecord, error)             { return nil, nil }
func (m *mockStore) GetHistoryByTrace(traceID string) ([]*store.ExecutionRecord, error) { return nil, nil }

// ── MOCK CLIENTS (for resolver) ───────────────────────────────────────────────

type mockNexus struct{}
type mockAtlas struct{}

func (m *mockNexus) GetProject(_ context.Context, id string) (*nexusclient.Project, error) {
	return &nexusclient.Project{ID: id}, nil
}
func (m *mockAtlas) GetProject(_ context.Context, id string) (*atlasclient.ProjectDetail, error) {
	return &atlasclient.ProjectDetail{ID: id, Language: "go", Path: "/workspace/projects/" + id}, nil
}
func (m *mockAtlas) GetWorkspaceContext(_ context.Context) (*atlasclient.WorkspaceContext, error) {
	return &atlasclient.WorkspaceContext{WorkspaceRoot: "/home/harsh/workspace"}, nil
}

// ── MOCK INTENT HANDLER ───────────────────────────────────────────────────────

type alwaysSuccessHandler struct{ intentName string }

func (h *alwaysSuccessHandler) Intent() string { return h.intentName }
func (h *alwaysSuccessHandler) Execute(_ context.Context, cmd *command.Command) *intent.Result {
	return &intent.Result{
		CommandID: cmd.ID,
		Intent:    cmd.Intent,
		Target:    cmd.Target,
		Success:   true,
		Output:    "ok",
	}
}

type alwaysFailHandler struct{ intentName string }

func (h *alwaysFailHandler) Intent() string { return h.intentName }
func (h *alwaysFailHandler) Execute(_ context.Context, cmd *command.Command) *intent.Result {
	return &intent.Result{
		CommandID: cmd.ID,
		Intent:    cmd.Intent,
		Target:    cmd.Target,
		Success:   false,
		Error:     "simulated failure",
	}
}

// ── HELPERS ──────────────────────────────────────────────────────────────────

func baseCtx() command.CommandContext {
	return command.CommandContext{
		WorkspaceRoot:   "/home/harsh/workspace",
		RequestingAgent: "test",
		Timestamp:       time.Now(),
	}
}

func newTestExecutor(s store.Storer, e *executor.Engine) *Executor {
	resolver := forgecontext.NewResolver(
		&mockNexus{}, &mockAtlas{},
		log.New(os.Stderr, "[test] ", 0),
	)
	return NewExecutor(s, e, resolver, log.New(os.Stderr, "[test] ", 0))
}

// ── TESTS ─────────────────────────────────────────────────────────────────────

func TestExecutor_RunsAllSteps(t *testing.T) {
	s := newMockStore()
	wf := &store.Workflow{ID: "wf-001", Name: "test-workflow", Trigger: "manual"}
	s.workflows["wf-001"] = wf
	s.steps["wf-001"] = []*store.WorkflowStep{
		{WorkflowID: "wf-001", Position: 1, Intent: command.IntentBuild, Target: "nexus", Parameters: map[string]string{}},
		{WorkflowID: "wf-001", Position: 2, Intent: command.IntentTest, Target: "nexus", Parameters: map[string]string{}},
	}

	e := executor.NewEngine()
	e.Register(&alwaysSuccessHandler{intentName: command.IntentBuild})
	e.Register(&alwaysSuccessHandler{intentName: command.IntentTest})

	ex := newTestExecutor(s, e)
	result, err := ex.Run(context.Background(), "wf-001", baseCtx())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.StepsDone != 2 {
		t.Errorf("StepsDone = %d, want 2", result.StepsDone)
	}
}

func TestExecutor_StopsOnFirstFailure(t *testing.T) {
	s := newMockStore()
	s.workflows["wf-002"] = &store.Workflow{ID: "wf-002", Name: "fail-workflow", Trigger: "manual"}
	s.steps["wf-002"] = []*store.WorkflowStep{
		{WorkflowID: "wf-002", Position: 1, Intent: command.IntentBuild, Target: "nexus", Parameters: map[string]string{}},
		{WorkflowID: "wf-002", Position: 2, Intent: command.IntentTest, Target: "nexus", Parameters: map[string]string{}},
		{WorkflowID: "wf-002", Position: 3, Intent: command.IntentDeploy, Target: "nexus", Parameters: map[string]string{}},
	}

	e := executor.NewEngine()
	e.Register(&alwaysFailHandler{intentName: command.IntentBuild}) // step 1 fails
	e.Register(&alwaysSuccessHandler{intentName: command.IntentTest})
	e.Register(&alwaysSuccessHandler{intentName: command.IntentDeploy})

	ex := newTestExecutor(s, e)
	result, err := ex.Run(context.Background(), "wf-002", baseCtx())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure when first step fails")
	}
	if result.StepsDone != 1 {
		t.Errorf("StepsDone = %d, want 1 (stopped after first failure)", result.StepsDone)
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestExecutor_WorkflowNotFound(t *testing.T) {
	s := newMockStore()
	e := executor.NewEngine()
	ex := newTestExecutor(s, e)

	_, err := ex.Run(context.Background(), "nonexistent", baseCtx())
	if err == nil {
		t.Error("expected error for nonexistent workflow")
	}
}

func TestExecutor_StoreError(t *testing.T) {
	s := newMockStore()
	s.err = errors.New("db error")
	e := executor.NewEngine()
	ex := newTestExecutor(s, e)

	_, err := ex.Run(context.Background(), "wf-001", baseCtx())
	if err == nil {
		t.Error("expected error when store fails")
	}
}

func TestCreateWorkflowRequest_Validate(t *testing.T) {
	cases := []struct {
		name    string
		req     CreateWorkflowRequest
		wantErr bool
	}{
		{"valid", CreateWorkflowRequest{
			Name:  "deploy-nexus",
			Steps: []StepInput{{Intent: "build", Target: "nexus"}},
		}, false},
		{"missing name", CreateWorkflowRequest{
			Steps: []StepInput{{Intent: "build", Target: "nexus"}},
		}, true},
		{"no steps", CreateWorkflowRequest{Name: "x"}, true},
		{"step missing intent", CreateWorkflowRequest{
			Name:  "x",
			Steps: []StepInput{{Target: "nexus"}},
		}, true},
		{"step missing target", CreateWorkflowRequest{
			Name:  "x",
			Steps: []StepInput{{Intent: "build"}},
		}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			if tc.wantErr && err == nil {
				t.Error("expected validation error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
