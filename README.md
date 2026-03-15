# Forge

Workflow Execution Engine — Developer Platform Execution Domain

---

## What Forge Is

Forge is the execution domain of the developer platform.
It translates developer intent into coordinated actions across the platform.

Forge acts. Nexus coordinates. Atlas understands.

---

## Position in Platform

```
Control    Nexus   coordinates the system
Knowledge  Atlas   understands the system
Execution  Forge   acts on the system     ← this project
```

---

## Responsibilities

**Phase 1 — Command Execution**
- Accept structured command objects from CLI or HTTP API
- Validate, resolve context, execute intent
- Report results

**Phase 2 — Workflow Definitions**
- Named sequences of commands, stored and reusable
- Manual or triggered execution

**Phase 3 — Automation Triggers**
- Event-driven execution
- Workspace events trigger workflow runs

---

## Command Object

```json
{
  "id":         "<uuid>",
  "intent":     "build",
  "target":     "nexus",
  "parameters": { "flags": "-race" },
  "context":    { "workspace_root": "/home/harsh/workspace" }
}
```

---

## API

```
POST http://127.0.0.1:8082/commands      submit a command
GET  http://127.0.0.1:8082/commands/:id  retrieve result
GET  http://127.0.0.1:8082/intents       list supported intents
GET  http://127.0.0.1:8082/health        liveness probe
```

---

## CLI (via engx)

```bash
engx run build nexus
engx run test nexus
engx run deploy nexus
```

---

## Build

```bash
go build -o ~/bin/forge ./cmd/forge/
```

---

## Architecture

See `architecture/forge-specification.md`

Platform-wide rules: `~/workspace/architecture/`
