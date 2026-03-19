// @forge-project: forge
// @forge-path: internal/trigger/scheduler.go
// CronScheduler fires workflows on a time-based schedule (Phase 5 / ADR-027).
//
// Supported schedule expressions (keep it simple — no full cron syntax):
//   @every 30m    — fire every 30 minutes
//   @every 1h     — fire every 1 hour
//   @every 6h     — fire every 6 hours
//   @hourly       — alias for @every 1h
//   @daily        — alias for @every 24h
//
// Design decisions:
//   - No external cron library. time.ParseDuration covers all practical needs.
//   - Each scheduled trigger gets its own ticker goroutine, bounded by the
//     same semaphore as the event Subscriber (maxConcurrentWorkflows = 8).
//   - Store is polled at startup and re-polled every 60s — new triggers
//     added via POST /triggers are picked up without a restart.
//   - Triggers with an empty Schedule field are ignored by the scheduler.
//   - A scheduler tick fires the workflow with target="" and
//     requesting_agent="scheduler" in the command context.
package trigger

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Harshmaury/Forge/internal/store"
	"github.com/Harshmaury/Forge/internal/workflow"
)

const schedulerRefreshInterval = 60 * time.Second

// CronScheduler polls the store for scheduled triggers and fires them on time.
type CronScheduler struct {
	store    store.Storer
	executor *workflow.Executor
	logger   *log.Logger
	sem      chan struct{} // shared semaphore — same bound as Subscriber
}

// NewCronScheduler creates a CronScheduler sharing the given semaphore.
func NewCronScheduler(
	s store.Storer,
	executor *workflow.Executor,
	logger *log.Logger,
	sem chan struct{},
) *CronScheduler {
	return &CronScheduler{
		store:    s,
		executor: executor,
		logger:   logger,
		sem:      sem,
	}
}

// Run starts the scheduling loop and blocks until ctx is cancelled.
// It refreshes the trigger list every 60s so newly registered cron
// triggers are picked up without a process restart.
func (cs *CronScheduler) Run(ctx context.Context) error {
	if err := cs.startTriggerTickers(ctx); err != nil {
		cs.logger.Printf("WARNING: cron scheduler initial load failed: %v", err)
	}
	refresh := time.NewTicker(schedulerRefreshInterval)
	defer refresh.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-refresh.C:
			if err := cs.startTriggerTickers(ctx); err != nil {
				cs.logger.Printf("WARNING: cron scheduler refresh: %v", err)
			}
		}
	}
}

// startTriggerTickers loads all cron triggers and starts a goroutine per trigger.
// Duplicate tickers for the same trigger ID are harmless — each goroutine
// stops when ctx is cancelled, so the worst case is one extra tick per
// refresh interval before the old goroutine exits.
func (cs *CronScheduler) startTriggerTickers(ctx context.Context) error {
	triggers, err := cs.store.GetEnabledCronTriggers()
	if err != nil {
		return fmt.Errorf("load cron triggers: %w", err)
	}
	for _, t := range triggers {
		interval, err := parseSchedule(t.Schedule)
		if err != nil {
			cs.logger.Printf("WARNING: trigger %s has invalid schedule %q: %v", t.ID, t.Schedule, err)
			continue
		}
		go cs.runTicker(ctx, t, interval)
	}
	cs.logger.Printf("cron scheduler: loaded %d scheduled trigger(s)", len(triggers))
	return nil
}

// runTicker fires the trigger's workflow on every interval tick until ctx ends.
func (cs *CronScheduler) runTicker(ctx context.Context, t *store.Trigger, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	cs.logger.Printf("cron trigger %s: schedule=%s interval=%s workflow=%s",
		t.ID, t.Schedule, interval, t.WorkflowID)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cs.dispatch(ctx, t)
		}
	}
}

// dispatch fires one workflow execution for a scheduled trigger.
// Uses the semaphore to cap concurrent executions (same bound as Subscriber).
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

// fireWorkflow resolves and executes the workflow for trigger t.
func (cs *CronScheduler) fireWorkflow(ctx context.Context, t *store.Trigger) {
	wf, err := cs.store.GetWorkflow(t.WorkflowID)
	if err != nil || wf == nil {
		cs.logger.Printf("WARNING: cron trigger %s: workflow %s not found", t.ID, t.WorkflowID)
		return
	}
	cs.logger.Printf("cron trigger %s: firing workflow %s (%s)", t.ID, wf.Name, t.Schedule)
	if err := cs.executor.Run(ctx, wf, "scheduler"); err != nil {
		cs.logger.Printf("WARNING: cron trigger %s: workflow %s failed: %v", t.ID, wf.Name, err)
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
	if after, ok := strings.CutPrefix(strings.TrimSpace(s), "@every "); ok {
		d, err := time.ParseDuration(strings.TrimSpace(after))
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", after, err)
		}
		if d < time.Minute {
			return 0, fmt.Errorf("schedule interval %s is too short — minimum is 1m", d)
		}
		return d, nil
	}
	return 0, fmt.Errorf("unrecognised schedule %q — use @every <dur>, @hourly, or @daily", s)
}
