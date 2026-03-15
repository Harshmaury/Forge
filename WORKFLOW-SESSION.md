# WORKFLOW-SESSION.md
# @version: 3.0.0
# @updated: 2026-03-16
# @repo: https://github.com/Harshmaury/Forge

---

## START A SESSION

```bash
cd ~/workspace/projects/apps/forge && ./scripts/verify.sh
```

Paste the output into Claude. Claude reads hash, confirms, asks for task.

---

## SESSION KEY

Format: `FG-<git-short-hash>-<YYYYMMDD>`

---

## IDENTITY

Developer: Harsh Maury  |  GitHub: https://github.com/Harshmaury
OS: Ubuntu 24.04 (WSL2) + Windows 11
Go: 1.25.0  uuid: v1.6.0  SQLite: mattn/go-sqlite3  YAML: yaml.v3

---

## PLATFORM

```
Control    Nexus   ~/workspace/projects/apps/nexus   :8080
Knowledge  Atlas   ~/workspace/projects/apps/atlas   :8081
Execution  Forge   ~/workspace/projects/apps/forge   :8082  <- this
```

Forge acts. Nexus coordinates. Atlas understands.

---

## BUILD STATUS
# Last verified: 2026-03-16

✅ Phase 1  Command execution (build, test, run, deploy)
✅ Phase 2  Workflow definitions + executor
✅ Phase 3  Automation triggers (event-to-workflow)
✅ v0.4.0-fixes-complete  All criticals + highs resolved

Tag: v0.4.0-fixes-complete -> commit 018a833

---

## API ENDPOINTS

```
POST /commands              submit a command
GET  /intents               list registered intent handlers
POST /workflows             create workflow
GET  /workflows             list workflows
GET  /workflows/:id         get workflow + steps
POST /workflows/:id/run     execute workflow
POST /triggers              register event-to-workflow trigger
GET  /triggers              list triggers
DELETE /triggers/:id        remove trigger
GET  /health
```

---

## ENVIRONMENT VARIABLES

  FORGE_HTTP_ADDR    :8082
  FORGE_WORKSPACE    ~/workspace
  NEXUS_HTTP_ADDR    http://127.0.0.1:8080
  ATLAS_HTTP_ADDR    http://127.0.0.1:8081

---

## BUILD + RUN

```bash
go build -o ~/bin/forge ./cmd/forge/
~/bin/forge &
```

---

## ROADMAP

Phase 3 complete. Next work is ADR-driven.

---

## CHANGELOG

2026-03-16  v3.0.0  All criticals + highs fixed, tagged v0.4.0-fixes-complete
2026-03-15  v2.5.0  Phase 3 complete — automation triggers
2026-03-15  v2.1.0  Phase 2 complete — workflow definitions
2026-03-15  v1.8.0  Phase 1 complete — command execution
