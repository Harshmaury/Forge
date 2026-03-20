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
func (m *schedulerMockStore) Close() error                                             { return nil }
func (m *schedulerMockStore) CreateWorkflow(w *store.Workflow) error                   { return nil }
func (m *schedulerMockStore) GetAllWorkflows() ([]*store.Workflow, error)              { return nil, nil }
func (m *schedulerMockStore) DeleteWorkflow(id string) error                           { return nil }
func (m *schedulerMockStore) WithWorkflowTransaction(fn func() error) error            { return fn() }
func (m *schedulerMockStore) AddStep(s *store.WorkflowStep) error                      { return nil }
func (m *schedulerMockStore) GetSteps(id string) ([]*store.WorkflowStep, error)        { return nil, nil }
func (m *schedulerMockStore) DeleteSteps(id string) error                              { return nil }
func (m *schedulerMockStore) CreateTrigger(t *store.Trigger) error                     { return nil }
func (m *schedulerMockStore) GetTrigger(id string) (*store.Trigger, error)             { return nil, nil }
func (m *schedulerMockStore) GetAllTriggers() ([]*store.Trigger, error)                { return nil, nil }
func (m *schedulerMockStore) GetEnabledTriggersByEvent(e string) ([]*store.Trigger, error) {
	return nil, nil
}
func (m *schedulerMockStore) DeleteTrigger(id string) error { return nil }
func (m *schedulerMockStore) LogExecution(r *store.ExecutionRecord) error { return nil }
func (m *schedulerMockStore) GetHistory(limit int) ([]*store.ExecutionRecord, error) {
	return nil, nil
}
func (m *schedulerMockStore) GetHistoryByTrace(id string) ([]*store.ExecutionRecord, error) {
	return nil, nil
}

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
		store:    s,
		logger:   logger,
		sem:      sem,
		active:   make(map[string]*tickerEntry),
	}
	return cs, sem
}

func trigger1m(id, workflowID string) *store.Trigger {
	return &store.Trigger{ID: id, WorkflowID: workflowID, Schedule: "@every 1m", Enabled: true}
}

// ── TESTS ─────────────────────────────────────────────────────────────────────

// TestReconcile_NoDuplicateTickers verifies that calling reconcile multiple
// times for the same trigger IDs starts exactly one goroutine per trigger,
// not one per reconcile call.
func TestReconcile_NoDuplicateTickers(t *testing.T) {
	s := &schedulerMockStore{}
	s.setTriggers([]*store.Trigger{
		trigger1m("t1", "wf-1"),
		trigger1m("t2", "wf-2"),
	})

	cs, _ := newTestScheduler(s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Call reconcile 5 times — simulates 5 refresh cycles.
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

// TestReconcile_StopsRemovedTrigger verifies that a trigger removed from the
// store is stopped on the next reconcile cycle.
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

	// Remove the trigger from store.
	s.setTriggers(nil)
	cs.reconcile(ctx)

	cs.mu.Lock()
	activeCount := len(cs.active)
	cs.mu.Unlock()

	if activeCount != 0 {
		t.Errorf("expected 0 active tickers after trigger removed, got %d", activeCount)
	}
}

// TestReconcile_RestartsOnScheduleChange verifies that when a trigger's
// schedule string changes, the old goroutine is cancelled and a new one started.
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

	// Change schedule.
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

// TestStopAll verifies that stopAll cancels all running tickers and empties
// the active map — called on context cancellation (daemon shutdown).
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

// TestParseSchedule covers all valid and invalid schedule expressions.
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

// TestSemaphoreDropsWhenFull verifies that dispatch does not block when the
// semaphore is full — it drops the tick with a WARNING log instead.
func TestSemaphoreDropsWhenFull(t *testing.T) {
	s := &schedulerMockStore{}
	sem := make(chan struct{}, 1)
	// Fill the semaphore completely.
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

	// dispatch should return immediately without blocking.
	done := make(chan struct{})
	go func() {
		cs.dispatch(ctx, t1)
		close(done)
	}()

	select {
	case <-done:
		// correct — returned without blocking
	case <-time.After(500 * time.Millisecond):
		t.Error("dispatch blocked when semaphore was full — expected immediate drop")
	}
}
