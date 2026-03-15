# WORKFLOW-ARCH.md
# @version: 2.0.0
# @updated: 2026-03-16

---

## LAYER MAP

```
cmd/forge/main.go         Entry point — wires all components
internal/api/             HTTP server :8082, handlers
internal/command/         Command model (ADR-004), validator, translator
internal/executor/        Execution engine, intent handlers (build/test/run/deploy)
internal/workflow/        Workflow definition storage + step executor
internal/trigger/         Event-to-workflow registry, subscriber, filter matching
internal/context/         Context resolver — enriches from Atlas + Nexus once per run
internal/store/           SQLite, Storer interface, versioned migrations
internal/nexus/           Nexus HTTP client
internal/atlas/           Atlas HTTP client
internal/config/          Env helpers
```

---

## PLATFORM RULES

ADR-001  Nexus owns project registry.
         Forge resolves targets via GET /projects/:id — never maintains own list.

ADR-003  HTTP/JSON on 127.0.0.1:8082.
         Response envelope: { ok, data, error }

ADR-004  Command model is the core abstraction.
         Five required fields: id, intent, target, parameters, context.
         All input is translated to Command before the executor sees it.

---

## DESIGN RULES

1. Translation first — every entry point passes through the translator.
2. Executor receives Command objects only — never raw strings.
3. Service lifecycle goes through Nexus HTTP API — Forge never calls providers.
4. Context enrichment (ResolveContext) runs once per workflow run, not per step.
5. Concurrent trigger dispatch bounded by semaphore (maxConcurrentWorkflows = 8).
6. Workflow creation is atomic via WithWorkflowTransaction.
7. SupportedEvents keyed by nexusevents.Topic — type-safe map lookup.
8. All migrations in store/db.go allMigrations slice — never in init().
9. FORGE_WORKSPACE read from env — never hardcoded.

---

## AI CODING RULES

BEFORE WRITING CODE:
  State understanding in 2 lines
  List every file to create or modify
  Grep all import usages before adding or removing any import
  Wait for approval

FILE NAMING:
  Format:  forge_<package>_<filename>__<YYYYMMDD>_<HHMM>.go
  Line 1:  // @forge-project: forge
  Line 2:  // @forge-path: <relative/path/to/file.go>

CODE STANDARDS:
  SOLID — no exceptions
  Max 40 lines per function
  All errors handled explicitly
  Named constants — no magic numbers
  Dependency injection — no package-level mutable state
  Interfaces over concrete types

TESTING:
  Mock HTTP clients for Nexus and Atlas calls
  Table-driven tests for intent handler validation

---

## DROP FOLDER

All deliveries go to: C:\Users\harsh\Downloads\engx-drop\
WSL2:                 /mnt/c/Users/harsh/Downloads/engx-drop/

---

## WHAT FORGE MUST NEVER DO

- Start or stop services directly (goes through Nexus)
- Scan the filesystem (uses Atlas HTTP API)
- Maintain a canonical project list independently of Nexus
- Pass raw strings to the execution engine
- Call ResolveContext more than once per workflow run
- Spawn unbounded goroutines on trigger dispatch
