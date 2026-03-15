# WORKFLOW-ARCH.md
# Forge architecture rules and AI coding standards
# @version: 1.0.0
# @updated: 2026-03-15

---

## LAYER MAP

```
cmd/forge/main.go         Entry point — wires all components
internal/api/             HTTP server and handlers (port 8082)
internal/command/         Command object model, validation, translation
internal/executor/        Execution pipeline and intent handlers
internal/context/         Context resolution from Atlas + Nexus
internal/nexus/           Nexus HTTP client
internal/atlas/           Atlas HTTP client
internal/store/           SQLite (Phase 2 workflow storage)
internal/config/          Env helpers
```

---

## PLATFORM RULES (from ADRs)

ADR-001: Nexus owns project registry
  Forge resolves targets via GET /projects/:id — never maintains own list

ADR-003: HTTP/JSON on 127.0.0.1:8082
  Response envelope: { ok, data, error }

ADR-004: Command model is core abstraction
  Five required fields: id, intent, target, parameters, context
  All input translated to Command before execution engine sees it

---

## FORGE-SPECIFIC DESIGN RULES

1. Translation first — every entry point passes through the translator
2. Executor receives Command objects only — never raw strings or flags
3. Delegate to Nexus — service lifecycle calls go through Nexus HTTP API
4. Enrich from Atlas — context resolution queries Atlas Phase 1 API
5. Stateless Phase 1 — no persistence until Phase 2 begins
6. Phase gate — Phase 2 workflow storage builds on proven Phase 1 execution
7. id always present — Forge generates uuid if caller does not supply one

---

## AI CODING RULES

BEFORE WRITING CODE:
  State understanding in 2 lines
  List every file to create or modify
  Wait for approval

FILE NAMING:
  Format:  forge_<package>_<filename>__<YYYYMMDD>_<HHMM>.go
  Example: forge_executor_engine__20260315_0900.go
  Line 1:  // @forge-project: forge
  Line 2:  // @forge-path: internal/executor/engine.go

CODE STANDARDS:
  SOLID — no exceptions
  Max 40 lines per function
  All errors handled explicitly
  Named constants — no magic numbers
  Dependency injection everywhere
  Interfaces over concrete types

TESTING:
  Every new component gets a test file
  Mock HTTP clients for Nexus and Atlas calls
  Table-driven tests for intent handler validation

---

## COMMAND TRANSLATION PATTERN

```go
// Every entry point uses the translator
func (h *CommandHandler) Submit(w http.ResponseWriter, r *http.Request) {
    var raw RawCommandRequest
    json.NewDecoder(r.Body).Decode(&raw)

    // Translation layer — always happens before executor
    cmd, err := h.translator.Translate(r.Context(), raw)
    if err != nil {
        respondErr(w, http.StatusBadRequest, err)
        return
    }

    // Executor always receives a validated Command object
    result, err := h.executor.Execute(r.Context(), cmd)
    ...
}
```

---

## WHAT FORGE MUST NEVER DO

- Start or stop services directly (use Nexus HTTP API)
- Scan the filesystem (use Atlas HTTP API for context)
- Maintain a canonical project list independently of Nexus
- Pass raw strings to the execution engine
- Build Phase 2 workflow storage before Phase 1 execution is proven
- Import Nexus internal packages other than eventbus (Phase 3 only)
