# WORKFLOW-SESSION.md
# Session: FG-phase5-scheduled-triggers
# Date: 2026-03-19

## What changed — Forge Phase 5 (ADR-027)

Scheduled (cron) triggers. POST /triggers now accepts a schedule field.
A CronScheduler runs alongside the event Subscriber, sharing the same
semaphore (max 8 concurrent workflows). Schedule expressions:
  @every 30m | @every 1h | @every 6h | @hourly | @daily

## New files
- internal/trigger/scheduler.go    — CronScheduler, parseSchedule(), ticker loop

## Modified files
- internal/store/storer.go         — Schedule field on Trigger struct
                                     GetEnabledCronTriggers() in interface
- internal/store/db.go             — migration v5: schedule column
                                     GetEnabledCronTriggers() implementation
                                     SELECT/INSERT/scan updated for schedule
- internal/trigger/model.go        — Schedule in CreateTriggerRequest
                                     Validate: event OR schedule required
                                     ToStoreTrigger: carries Schedule
- internal/trigger/subscriber.go   — Sem() method exported
- nexus.yaml                       — version: 0.6.0

## Apply

cd ~/workspace/projects/apps/forge && \
unzip -o /mnt/c/Users/harsh/Downloads/engx-drop/forge-phase5-scheduled-triggers-20260319.zip -d .

Then apply MAIN_GO_PATCH.md to cmd/forge/main.go (2 changes, script inside the file).

go build ./...

## Verify

go build ./... && go test ./internal/trigger/...

# Register a scheduled trigger:
curl -s -X POST http://127.0.0.1:8082/triggers \
  -H "Content-Type: application/json" \
  -d '{"workflow_id":"<id>","schedule":"@every 5m"}' | jq .

# Check it's listed:
curl -s http://127.0.0.1:8082/triggers | jq '.data.triggers[] | {id, schedule}'

# After 5 minutes, check forge history:
curl -s http://127.0.0.1:8082/history | jq '.data[0] | {intent, target, status}'

# Via engx:
engx trigger add "" <workflow-id> --schedule "@every 5m"

## Commit

git add \
  internal/trigger/scheduler.go \
  internal/trigger/model.go \
  internal/trigger/subscriber.go \
  internal/store/storer.go \
  internal/store/db.go \
  cmd/forge/main.go \
  nexus.yaml \
  WORKFLOW-SESSION.md && \
git commit -m "feat(phase5): scheduled cron triggers — @every, @hourly, @daily (ADR-027)" && \
git tag v0.6.0-phase5 && \
git push origin main --tags
