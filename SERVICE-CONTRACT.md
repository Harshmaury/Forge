// @forge-project: forge
// @forge-path: SERVICE-CONTRACT.md
# SERVICE-CONTRACT.md — Forge
# @version: 0.5.0-phase5
# @updated: 2026-03-25

**Port:** 8082 · **DB:** `~/.nexus/forge.db` · **Domain:** Execution

---

## Code

```
cmd/forge/main.go                startup wiring
internal/command/model.go        Command struct — {id, intent, target, parameters, context}
internal/command/validator.go    validates raw input
internal/command/translator.go   translates to Command
internal/executor/engine.go      dispatches to intent handlers
internal/executor/intent/        run.go · build.go · deploy.go · test.go
internal/preflight/checker.go    queries Atlas before execution (fail-open)
internal/workflow/               workflow model + executor
internal/trigger/scheduler.go    cron + event trigger dispatch (semaphore: max 8)
internal/store/db.go             Storer, SQLite, versioned migrations (v6: command_dedup)
internal/api/handler/history.go  GET /history, GET /history/:trace_id
```

---

## Contract

### Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/health` | none | `{"ok":true,"status":"healthy","service":"forge"}` |
| POST | `/commands` | token | Submit command — returns result or `409` on duplicate |
| GET | `/history` | token | Paginated execution history — `ForgeExecutionDTO[]` |
| GET | `/history/:trace_id` | token | Execution records for one trace |
| GET | `/workflows` | token | All defined workflows |
| POST | `/workflows` | token | Define workflow |
| GET | `/triggers` | token | All registered triggers |
| POST | `/triggers` | token | Register automation trigger |

### Command dedup

Caller supplies explicit `command_id`:
- First call: `200 OK` + result
- Retry within 300s TTL (same id): `409` + body `{"ok":false,"data":<original result>}`
- No `command_id` supplied: UUID generated — never a duplicate

### Execution record schema

`ForgeExecutionDTO`: `id`, `command_id`, `intent`, `target`, `trace_id`, `status`, `output`, `error`, `duration_ms`, `started_at`, `finished_at`, `actor_sub`, `actor_scope`.

### Failure conditions

| Code | Condition |
|------|-----------|
| 400 | Invalid command input |
| 401 | Missing or invalid `X-Service-Token` |
| 404 | Workflow or trigger not found |
| 409 | Duplicate command within TTL |

---

## Control

**Command lifecycle:**
1. `POST /commands` → `validator.Validate()` → `translator.Translate()` → `Command`
2. Dedup check against `command_dedup` table (TTL 300s)
3. `preflight.Checker.Check()` — `Atlas GET /graph/services` — fail-open
4. `executor.Execute()` → intent handler
5. Intent handler: `Nexus POST /projects/:id/start|stop`
6. Result → `execution_history`

**Trigger dispatch:** cron ticks + event poll every 3s. Semaphore: max 8 concurrent goroutines. Dropped triggers logged at WARNING.

**PreflightSnapshot:** captured at `checker.Check()` call time, passed by value — immutable through the execution pipeline (ADR-021).

---

## Context

- Sole service permitted to call `POST /projects/:id/start|stop` on Nexus.
- Does not call Guardian, Navigator, Observer, Metrics, or Sentinel.
- Does not scan the filesystem — queries Atlas for workspace context (ADR-006).
- Does not make AI calls.
