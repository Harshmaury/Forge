# WORKFLOW-SESSION.md
# Session: forge-fix-trigger-store-20260325
# Date: 2026-03-25

## What changed — Forge fix: BUG-1 + BUG-2 + BUG-3 (trigger store)

Three SQL bugs in `internal/store/db.go` that completely disabled cron triggers
(ADR-027 / Forge Phase 5): a placeholder count mismatch in `CreateTrigger`,
a column/scan destination mismatch in `GetEnabledCronTriggers`, and `schedule`
missing from all non-cron SELECT queries and scan helpers.

## New files
- (none)

## Modified files
- `internal/store/db.go`    — BUG-1: CreateTrigger VALUES placeholder count 8→9;
                              BUG-2: GetEnabledCronTriggers uses new scanCronTriggers;
                              BUG-3: schedule added to GetTrigger / GetAllTriggers /
                              GetEnabledTriggersByEvent SELECT + scanTriggers Scan;
                              scanCronTriggers() added

## Apply

cd ~/workspace/projects/engx/services/forge && \
unzip -o /mnt/c/Users/harsh/Downloads/engx-drop/forge-fix-trigger-store-20260325.zip -d . && \
go build ./...

## Verify

go test ./internal/store/...

# Quick smoke test (daemon must be running):
# curl -s -X POST http://127.0.0.1:8082/triggers \
#   -H "X-Service-Token: <token>" \
#   -H "Content-Type: application/json" \
#   -d '{"event":"","schedule":"@every 5m","workflow_id":"<id>"}' | jq

## Commit

git add internal/store/db.go WORKFLOW-SESSION.md && \
git commit -m "fix(trigger-store): BUG-1 CreateTrigger placeholder count, BUG-2 scanCronTriggers, BUG-3 schedule in all SELECTs" && \
git tag v0.6.1-fix-trigger-store && \
git push origin main --tags
