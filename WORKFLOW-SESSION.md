# WORKFLOW-SESSION.md
# @version: 3.0.0
# @updated: 2026-03-16
# @repo: https://github.com/Harshmaury/Forge

---

## Start a Session

```bash
cd ~/workspace/projects/apps/forge && ./scripts/verify.sh
```

Paste output into Claude. Session key format: `FG-<hash>-<YYYYMMDD>`

---

## Identity

Developer: Harsh Maury | OS: Ubuntu 24.04 (WSL2) + Windows 11
Go: 1.25.0 | Drop folder: /mnt/c/Users/harsh/Downloads/engx-drop/

---

## Platform

```
Control    Nexus  :8080
Knowledge  Atlas  :8081
Execution  Forge  :8082  ← this
```

Forge acts. Nexus coordinates. Atlas understands.

---

## Build Status
# Last verified: 2026-03-16

✅ Phase 1    Command execution (build, test, run, deploy)
✅ Phase 2    Workflow definitions + executor
✅ Phase 3    Automation triggers (event-to-workflow)
✅ ADR-008    Inter-service auth (outbound X-Service-Token)
✅ v0.4.0-fixes-complete  All criticals + highs resolved

---

## Environment Variables

```
FORGE_HTTP_ADDR         :8082
FORGE_WORKSPACE         ~/workspace
NEXUS_HTTP_ADDR         http://127.0.0.1:8080
ATLAS_HTTP_ADDR         http://127.0.0.1:8081
FORGE_SERVICE_TOKEN     from ~/.nexus/service-tokens
```

---

## API

```
POST /commands              submit a command
GET  /intents               list registered intent handlers
POST /workflows             create workflow (atomic — WithWorkflowTransaction)
GET  /workflows             list workflows
GET  /workflows/:id         get workflow + steps
POST /workflows/:id/run     execute workflow
POST /triggers              register event-to-workflow trigger
GET  /triggers              list triggers
DELETE /triggers/:id        remove trigger
GET  /health
```

---

## Key Files

```
internal/trigger/subscriber.go   poll() — sends X-Service-Token via header (ADR-008)
internal/workflow/executor.go    ResolveContext once per run (not per step)
internal/store/db.go             WithWorkflowTransaction — atomic workflow creation
internal/context/resolver.go     10s timeout per enrichment run
cmd/forge/main.go                FORGE_SERVICE_TOKEN wired into both clients
```

---

## Roadmap

Phase 3 complete. Next phase requires ADR.

---

## Commands

All commands in `~/workspace/developer-platform/RUNBOOK.md`.

---

## Changelog

2026-03-16  v3.0.0  All criticals + highs fixed, ADR-008 implemented
2026-03-15  v2.5.0  Phase 3 complete — automation triggers
2026-03-15  v2.1.0  Phase 2 complete — workflow definitions
2026-03-15  v1.8.0  Phase 1 complete — command execution
