# WORKFLOW-SESSION.md
# Session: FG-fix-scheduler-dedup
# Date: 2026-03-20

## What changed — Forge CronScheduler robustness fix

Three bugs fixed in one pass:

**Bug 1 — Duplicate tickers (silent, accumulating)**
reconcile() was called every 60s and started one goroutine per trigger per
call with no dedup check. After 10 minutes: 10 goroutines per trigger, all
firing simultaneously, all competing for the semaphore, all writing duplicate
execution history entries. The comment "harmless" in the original was wrong.

**Bug 2 — CronScheduler never wired in main.go**
MAIN_GO_PATCH.md from phase5 was never applied. CronScheduler existed but
was never started — scheduled triggers registered successfully via
POST /triggers but never fired.

**Bug 3 — errCh buffer too small**
main.go had errCh buffer of 2 (api + subscriber). Adding the scheduler
goroutine required bumping to 3 to avoid a blocked send on shutdown.

## Fix design

CronScheduler gains an `active map[string]*tickerEntry` protected by a mutex.
Each entry holds the trigger's cancel function and schedule string.

reconcile() (renamed from startTriggerTickers):
- Stops goroutines for triggers no longer in the store or with changed schedules
- Starts goroutines only for triggers not already in active map
- Guarantees exactly one goroutine per active trigger at all times

On ctx cancellation: stopAll() cancels all active entries and empties the map.

Schedule changes are handled automatically: if a trigger's schedule string
differs from what's in the active map, the old goroutine is cancelled and a
new one started with the new interval — no restart required.

## Files changed

- `internal/trigger/scheduler.go`  — dedup map, reconcile(), stopAll(), per-ticker cancel
- `internal/trigger/scheduler_test.go` — 6 table-driven tests (new file)
- `cmd/forge/main.go`               — wire CronScheduler, errCh buffer 2 → 3, version bump

## Apply

```bash
cd ~/workspace/projects/engx/services/forge && \
unzip -o /mnt/c/Users/harsh/Downloads/engx-drop/forge-fix-scheduler-dedup-20260320-1530.zip -d . && \
go build ./... && \
go test ./internal/trigger/...
```

## Verify

```bash
go build ./...
go test ./internal/trigger/...
# Expected: all scheduler tests pass

# Register a cron trigger then check logs:
FORGE_SERVICE_TOKEN=<token> forge &
curl -s -X POST http://127.0.0.1:8082/triggers \
  -H "Content-Type: application/json" \
  -H "X-Service-Token: <token>" \
  -d '{"workflow_id":"<id>","schedule":"@every 1m"}' | jq .

# After 60s, check forge logs — should see exactly ONE fire per minute:
# [forge] cron trigger <id>: firing workflow test-workflow (@every 1m)
# NOT: multiple identical lines per minute
```

## Commit

```bash
git add \
  internal/trigger/scheduler.go \
  internal/trigger/scheduler_test.go \
  cmd/forge/main.go \
  WORKFLOW-SESSION.md && \
git commit -m "fix(scheduler): dedup active tickers + wire CronScheduler in main" && \
git push origin main
```
