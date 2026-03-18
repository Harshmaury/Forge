# SERVICE-CONTRACT.md — Forge

**Service:** forge
**Domain:** Execution
**Port:** 8082
**ADRs:** ADR-004 (intent model), ADR-005 (lifecycle protocol), ADR-006 (Atlas context),
         ADR-007 (automation triggers), ADR-008 (auth), ADR-010 (preflight + history),
         ADR-021 (execution context snapshot)
**Version:** 0.5.0-phase4
**Updated:** 2026-03-18

---

## Role

Execution engine. Translates developer intent into coordinated platform
actions. Every input becomes a Command object before the executor sees it.
Forge acts on the system — it is the only service permitted to instruct
Nexus to start or stop services.

---

## Inputs

- `POST /commands` — raw command request (translated to Command before execution)
- `POST /workflows` — workflow definition
- `POST /triggers` — automation trigger registration
- `Nexus GET /events?since=<id>` — workspace events for trigger matching
- `Atlas GET /graph/services` — verified project list for preflight validation

---

## Outputs

- `GET /health` — health check (no auth)
- `GET /history` — paginated execution history
- `GET /history/:trace_id` — execution records for one trace
- `GET /workflows` — all defined workflows
- `GET /triggers` — all registered triggers
- `POST /projects/:id/start|stop` — lifecycle instructions sent to Nexus
- Execution results returned in `POST /commands` response

---

## Dependencies

| Service | Used for                          | Auth required   |
|---------|-----------------------------------|-----------------|
| Nexus   | Lifecycle control + event polling | X-Service-Token |
| Atlas   | Preflight context + workspace ctx | X-Service-Token |

Forge does not depend on Guardian, Navigator, Observer, Metrics, or Sentinel.

---

## Guarantees

- All input becomes a `Command{id, intent, target, parameters, context}` before
  the executor sees it. No raw strings reach the executor (ADR-004).
- `ResolveContext` is called once per workflow run — not per step (ADR-006 Rule 4).
- Preflight check queries Atlas before execution. Fail-open if Atlas unreachable —
  a WARNING is logged and execution proceeds (ADR-010).
- `PreflightSnapshot` is captured at `checker.Check()` call time and passed
  by value — never re-queried between check and history log (ADR-021).
- Every execution (permitted, denied, failed) is logged to `execution_history`.
- Trigger dispatch is bounded to 8 concurrent workflow goroutines via semaphore.
  Dropped triggers are logged at WARNING level (ADR-007).
- All migrations are in a single ordered slice in `internal/store/db.go`.

---

## Non-Responsibilities

- Forge does not own the project registry — Nexus does (ADR-001).
- Forge does not index the workspace — Atlas does.
- Forge does not resolve context by scanning the filesystem — it queries Atlas (ADR-006).
- Forge does not evaluate policy findings — Guardian does.
- Forge does not call any observer service (Metrics, Navigator, Guardian,
  Observer, Sentinel).
- Forge does not make AI calls of any kind.

---

## Data Authority

**Primary authority for:**
- Command execution state — `~/.nexus/forge.db`
- Workflow definitions — `workflows` + `workflow_steps` tables
- Automation triggers — `triggers` table
- Execution history — `execution_history` table (including PreflightSnapshot)

---

## Concurrency Model

- SQLite store accessed through `store.Storer` interface.
- Trigger subscriber goroutine polls Nexus events independently.
- Workflow executor goroutines bounded by semaphore (max 8 concurrent).
- `X-Trace-ID` middleware propagates trace IDs on all responses.
- `PreflightSnapshot` passed by value — immutable across goroutine boundary.
