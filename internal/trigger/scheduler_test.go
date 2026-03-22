// @forge-project: forge
// @forge-path: internal/trigger/scheduler_test.go
package trigger

import (
	"context"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Harshmaury/Forge/internal/command"
	"github.com/Harshmaury/Forge/internal/store"
	"github.com/Harshmaury/Forge/internal/workflow"
)

// ── MOCK STORE ────────────────────────────────────────────────────────────────

type schedulerMockStore struct {
	mu       sync.Mutex
	triggers []*store.Trigger
	err      error
}

func (m *schedulerMockStore) setTriggers(t []*store.Trigger) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.triggers = t
}

func (m *schedulerMockStore) GetEnabledCronTriggers() ([]*store.Trigger, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	out := make([]*store.Trigger, len(m.triggers))
	copy(out, m.triggers)
	return out, nil
}

func (m *schedulerMockStore) GetWorkflow(id string) (*store.Workflow, error) {
	return &store.Workflow{ID: id, Name: "test-workflow"}, nil
}

// Satisfy full Storer interface — unused in scheduler tests.
func (m *schedulerMockStore) Close() error                                                        { return nil }
func (m *schedulerMockStore) CreateWorkflow(w *store.Workflow) error                              { return nil }
func (m *schedulerMockStore) GetAllWorkflows() ([]*store.Workflow, error)                         { return nil, nil }
func (m *schedulerMockStore) DeleteWorkflow(id string) error                                      { return nil }
func (m *schedulerMockStore) WithWorkflowTransaction(fn func() error) error                       { return fn() }
func (m *schedulerMockStore) AddStep(s *store.WorkflowStep) error                                 { return nil }
func (m *schedulerMockStore) GetSteps(id string) ([]*store.WorkflowStep, error)                   { return nil, nil }
func (m *schedulerMockStore) DeleteSteps(id string) error                                         { return nil }
func (m *schedulerMockStore) CreateTrigger(t *store.Trigger) error                                { return nil }
func (m *schedulerMockStore) GetTrigger(id string) (*store.Trigger, error)                        { return nil, nil }
func (m *schedulerMockStore) GetAllTriggers() ([]*store.Trigger, error)                           { return nil, nil }
func (m *schedulerMockStore) GetEnabledTriggersByEvent(e string) ([]*store.Trigger, error)        { return nil, nil }
func (m *schedulerMockStore) DeleteTrigger(id string) error                                       { return nil }
func (m *schedulerMockStore) LogExecution(r *store.ExecutionRecord) error                         { return nil }
func (m *schedulerMockStore) GetHistory(limit int) ([]*store.ExecutionRecord, error)              { return nil, nil }
func (m *schedulerMockStore) GetHistoryByTrace(id string) ([]*store.ExecutionRecord, error)       { return nil, nil }
// CW-5: dedup stubs — no-op for scheduler tests.
func (m *schedulerMockStore) GetDedupRecord(commandID string) (*store.DedupRecord, error)         { return nil, nil }
func (m *schedulerMockStore) SetDedupRecord(r *store.DedupRecord) error                           { return nil }

// ── COUNTING EXECUTOR ─────────────────────────────────────────────────────────

// countingExecutor counts how many times Run() is called per workflow.
type countingExecutor struct {
	mu     sync.Mutex
	counts map[string]int
}

func newCountingExecutor() *countingExecutor {
	return &countingExecutor{counts: make(map[string]int)}
}

func (c *countingExecutor) Run(_ context.Context, workflowID string, _ command.CommandContext) (*workflow.WorkflowRunResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counts[workflowID]++
	return &workflow.WorkflowRunResult{WorkflowID: workflowID, Success: true}, nil
}

func (c *countingExecutor) total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	sum := 0
	for _, v := range c.counts {
		sum += v
	}
	return sum
}

// ── HELPERS ───────────────────────────────────────────────────────────────────

func newTestScheduler(s *schedulerMockStore) (*CronScheduler, chan struct{}) {
	sem := make(chan struct{}, 8)
	logger := log.New(os.Stderr, "[test] ", 0)
	cs := &CronScheduler{
		store:  s,
		logger: logger,
		sem:    sem,
		active: make(map[string]*tickerEntry),
	}
	return cs, sem
}

func trigger1m(id, workflowID string) *store.Trigger {
	return &store.Trigger{ID: id, WorkflowID: workflowID, Schedule: "@every 1m", Enabled: true}
}

// ── TESTS ─────────────────────────────────────────────────────────────────────

func TestReconcile_NoDuplicateTickers(t *testing.T) {
	s := &schedulerMockStore{}
	s.setTriggers([]*store.Trigger{
		trigger1m("t1", "wf-1"),
		trigger1m("t2", "wf-2"),
	})

	cs, _ := newTestScheduler(s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i := 0; i < 5; i++ {
		cs.reconcile(ctx)
	}

	cs.mu.Lock()
	activeCount := len(cs.active)
	cs.mu.Unlock()

	if activeCount != 2 {
		t.Errorf("expected 2 active tickers after 5 reconciles, got %d", activeCount)
	}
}

func TestReconcile_StopsRemovedTrigger(t *testing.T) {
	s := &schedulerMockStore{}
	s.setTriggers([]*store.Trigger{trigger1m("t1", "wf-1")})

	cs, _ := newTestScheduler(s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cs.reconcile(ctx)

	cs.mu.Lock()
	if len(cs.active) != 1 {
		cs.mu.Unlock()
		t.Fatalf("expected 1 active ticker after first reconcile, got %d", len(cs.active))
	}
	cs.mu.Unlock()

	s.setTriggers(nil)
	cs.reconcile(ctx)

	cs.mu.Lock()
	activeCount := len(cs.active)
	cs.mu.Unlock()

	if activeCount != 0 {
		t.Errorf("expected 0 active tickers after trigger removed, got %d", activeCount)
	}
}

func TestReconcile_RestartsOnScheduleChange(t *testing.T) {
	s := &schedulerMockStore{}
	s.setTriggers([]*store.Trigger{trigger1m("t1", "wf-1")})

	cs, _ := newTestScheduler(s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cs.reconcile(ctx)

	cs.mu.Lock()
	oldEntry := cs.active["t1"]
	cs.mu.Unlock()
	if oldEntry == nil {
		t.Fatal("expected active entry for t1")
	}

	s.setTriggers([]*store.Trigger{
		{ID: "t1", WorkflowID: "wf-1", Schedule: "@every 2m", Enabled: true},
	})
	cs.reconcile(ctx)

	cs.mu.Lock()
	newEntry := cs.active["t1"]
	cs.mu.Unlock()

	if newEntry == nil {
		t.Fatal("expected active entry for t1 after schedule change")
	}
	if newEntry == oldEntry {
		t.Error("expected new ticker entry after schedule change, got same pointer")
	}
	if newEntry.schedule != "@every 2m" {
		t.Errorf("expected schedule @every 2m, got %q", newEntry.schedule)
	}
}

func TestStopAll(t *testing.T) {
	s := &schedulerMockStore{}
	s.setTriggers([]*store.Trigger{
		trigger1m("t1", "wf-1"),
		trigger1m("t2", "wf-2"),
		trigger1m("t3", "wf-3"),
	})

	cs, _ := newTestScheduler(s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cs.reconcile(ctx)

	cs.mu.Lock()
	count := len(cs.active)
	cs.mu.Unlock()
	if count != 3 {
		t.Fatalf("expected 3 active, got %d", count)
	}

	cs.stopAll()

	cs.mu.Lock()
	remaining := len(cs.active)
	cs.mu.Unlock()
	if remaining != 0 {
		t.Errorf("expected 0 active after stopAll, got %d", remaining)
	}
}

func TestParseSchedule(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantDur   time.Duration
		wantError bool
	}{
		{"hourly alias", "@hourly", time.Hour, false},
		{"daily alias", "@daily", 24 * time.Hour, false},
		{"every 30m", "@every 30m", 30 * time.Minute, false},
		{"every 2h", "@every 2h", 2 * time.Hour, false},
		{"every 1m exactly", "@every 1m", time.Minute, false},
		{"every 6h", "@every 6h", 6 * time.Hour, false},
		{"whitespace trimmed", "  @every 1h  ", time.Hour, false},
		{"too short — 30s", "@every 30s", 0, true},
		{"too short — 59s", "@every 59s", 0, true},
		{"invalid duration", "@every banana", 0, true},
		{"unknown keyword", "@nightly", 0, true},
		{"empty string", "", 0, true},
		{"bare duration no prefix", "30m", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSchedule(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("parseSchedule(%q) error = %v, wantError = %v", tt.input, err, tt.wantError)
			}
			if !tt.wantError && got != tt.wantDur {
				t.Errorf("parseSchedule(%q) = %v, want %v", tt.input, got, tt.wantDur)
			}
		})
	}
}

func TestSemaphoreDropsWhenFull(t *testing.T) {
	s := &schedulerMockStore{}
	sem := make(chan struct{}, 1)
	sem <- struct{}{}

	var logBuf atomic.Value
	logBuf.Store("")

	cs := &CronScheduler{
		store:  s,
		sem:    sem,
		logger: log.New(os.Stderr, "[test] ", 0),
		active: make(map[string]*tickerEntry),
	}

	ctx := context.Background()
	t1 := trigger1m("t1", "wf-1")

	done := make(chan struct{})
	go func() {
		cs.dispatch(ctx, t1)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Error("dispatch blocked when semaphore was full — expected immediate drop")
	}
}
