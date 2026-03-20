// @forge-project: forge
// @forge-path: internal/trigger/scheduler.go
// CronScheduler fires workflows on a time-based schedule (Phase 5 / ADR-027).
//
// Supported schedule expressions:
//   @every <dur>   — e.g. @every 30m, @every 2h  (minimum 1m)
//   @hourly        — alias for @every 1h
//   @daily         — alias for @every 24h
//
// Design:
//   - No external cron library. time.ParseDuration covers all practical needs.
//   - Each active trigger runs in exactly one goroutine. The active map tracks
//     running trigger IDs and their cancel functions. Refresh calls only start
//     goroutines for triggers not already running, and cancels goroutines for
//     triggers that were deleted or disabled since the last refresh.
//   - Shares the same semaphore as the event Subscriber — no parallel over-execution.
//   - Store is polled at startup and re-polled every 60s. New triggers are
//     picked up and removed triggers are stopped without a process restart.
//   - A scheduler tick fires the workflow with requesting_agent="scheduler".
package trigger

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Harshmaury/Forge/internal/command"
	"github.com/Harshmaury/Forge/internal/store"
	"github.com/Harshmaury/Forge/internal/workflow"
)

const schedulerRefreshInterval = 60 * time.Second

// tickerEntry tracks one running ticker goroutine.
type tickerEntry struct {
	cancel   context.CancelFunc
	schedule string
}

// WorkflowRunner is the execution contract the scheduler depends on.
// *workflow.Executor satisfies this interface. Tests supply a mock.
type WorkflowRunner interface {
	Run(ctx context.Context, workflowID string, baseContext command.CommandContext) (*workflow.WorkflowRunResult, error)
}

// CronScheduler polls the store for scheduled triggers and fires them on time.
// Each trigger runs in exactly one goroutine — guaranteed by the active map.
type CronScheduler struct {
	store    store.Storer
	executor WorkflowRunner
	logger   *log.Logger
	sem      chan struct{} // shared semaphore — same bound as Subscriber

	mu     sync.Mutex
	active map[string]*tickerEntry // triggerID → running goroutine entry
}

// NewCronScheduler creates a CronScheduler sharing the given semaphore.
func NewCronScheduler(
	s store.Storer,
	executor WorkflowRunner,
	logger *log.Logger,
	sem chan struct{},
) *CronScheduler {
	return &CronScheduler{
		store:    s,
		executor: executor,
		logger:   logger,
		sem:      sem,
		active:   make(map[string]*tickerEntry),
	}
}

// Run starts the scheduling loop and blocks until ctx is cancelled.
// On ctx cancellation, all active ticker goroutines are stopped cleanly.
func (cs *CronScheduler) Run(ctx context.Context) error {
	cs.reconcile(ctx)

	refresh := time.NewTicker(schedulerRefreshInterval)
	defer refresh.Stop()

	for {
		select {
		case <-ctx.Done():
			cs.stopAll()
			return nil
		case <-refresh.C:
			cs.reconcile(ctx)
		}
	}
}

// reconcile syncs running goroutines with the current store state.
// Starts goroutines for new/changed triggers, stops goroutines for
// deleted or disabled triggers, and restarts goroutines whose schedule changed.
func (cs *CronScheduler) reconcile(ctx context.Context) {
	triggers, err := cs.store.GetEnabledCronTriggers()
	if err != nil {
		cs.logger.Printf("WARNING: cron scheduler reconcile: %v", err)
		return
	}

	// Build a set of current trigger IDs → schedule for quick lookup.
	current := make(map[string]string, len(triggers))
	for _, t := range triggers {
		current[t.ID] = t.Schedule
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Stop goroutines for triggers no longer active or whose schedule changed.
	for id, entry := range cs.active {
		newSchedule, stillActive := current[id]
		if !stillActive || newSchedule != entry.schedule {
			entry.cancel()
			delete(cs.active, id)
			if !stillActive {
				cs.logger.Printf("cron scheduler: stopped trigger %s (removed or disabled)", id)
			} else {
				cs.logger.Printf("cron scheduler: restarting trigger %s (schedule changed: %s → %s)",
					id, entry.schedule, newSchedule)
			}
		}
	}

	// Start goroutines for new triggers not yet running.
	started := 0
	for _, t := range triggers {
		if _, running := cs.active[t.ID]; running {
			continue // already has exactly one goroutine
		}
		interval, err := parseSchedule(t.Schedule)
		if err != nil {
			cs.logger.Printf("WARNING: trigger %s has invalid schedule %q: %v — skipped", t.ID, t.Schedule, err)
			continue
		}
		tickCtx, cancel := context.WithCancel(ctx)
		cs.active[t.ID] = &tickerEntry{cancel: cancel, schedule: t.Schedule}
		go cs.runTicker(tickCtx, t, interval)
		started++
	}

	if started > 0 || len(cs.active) > 0 {
		cs.logger.Printf("cron scheduler: %d active trigger(s) (%d started this cycle)",
			len(cs.active), started)
	}
}

// stopAll cancels all running ticker goroutines. Called on shutdown.
func (cs *CronScheduler) stopAll() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	for id, entry := range cs.active {
		entry.cancel()
		delete(cs.active, id)
	}
}

// runTicker fires the trigger's workflow on every interval tick until ctx ends.
// When ctx is cancelled (by reconcile or shutdown), the ticker stops cleanly.
func (cs *CronScheduler) runTicker(ctx context.Context, t *store.Trigger, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	cs.logger.Printf("cron trigger %s started: schedule=%s interval=%s workflow=%s",
		t.ID, t.Schedule, interval, t.WorkflowID)
	for {
		select {
		case <-ctx.Done():
			cs.logger.Printf("cron trigger %s stopped", t.ID)
			return
		case <-ticker.C:
			cs.dispatch(ctx, t)
		}
	}
}

// dispatch fires one workflow execution for a scheduled trigger.
// Uses the semaphore to cap concurrent executions (same bound as Subscriber).
// Drops the tick if the semaphore is full rather than blocking.
func (cs *CronScheduler) dispatch(ctx context.Context, t *store.Trigger) {
	select {
	case cs.sem <- struct{}{}:
	default:
		cs.logger.Printf("WARNING: cron trigger %s dropped — semaphore full (max %d concurrent)",
			t.ID, cap(cs.sem))
		return
	}
	go func() {
		defer func() { <-cs.sem }()
		cs.fireWorkflow(ctx, t)
	}()
}

// fireWorkflow executes the workflow for trigger t.
func (cs *CronScheduler) fireWorkflow(ctx context.Context, t *store.Trigger) {
	cs.logger.Printf("cron trigger %s: firing workflow %s (%s)", t.ID, t.WorkflowID, t.Schedule)
	cmdCtx := command.CommandContext{
		RequestingAgent: "scheduler",
		Timestamp:       time.Now().UTC(),
	}
	if _, err := cs.executor.Run(ctx, t.WorkflowID, cmdCtx); err != nil {
		cs.logger.Printf("WARNING: cron trigger %s: workflow %s failed: %v", t.ID, t.WorkflowID, err)
	}
}

// ── SCHEDULE PARSING ──────────────────────────────────────────────────────────

// parseSchedule converts a schedule expression to a time.Duration.
// Supported: @every <duration>, @hourly, @daily.
func parseSchedule(s string) (time.Duration, error) {
	switch strings.TrimSpace(s) {
	case "@hourly":
		return time.Hour, nil
	case "@daily":
		return 24 * time.Hour, nil
	}
	after, ok := strings.CutPrefix(strings.TrimSpace(s), "@every ")
	if !ok {
		return 0, fmt.Errorf("unrecognised schedule %q — use @every <dur>, @hourly, or @daily", s)
	}
	d, err := time.ParseDuration(strings.TrimSpace(after))
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", after, err)
	}
	if d < time.Minute {
		return 0, fmt.Errorf("schedule interval %s is too short — minimum is 1m", d)
	}
	return d, nil
}
