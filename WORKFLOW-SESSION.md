# WORKFLOW-SESSION.md
# @version: 2.8.0
# @updated: 2026-03-16
# @repo: https://github.com/Harshmaury/Forge

---

## HOW TO START A SESSION

```bash
cd ~/workspace/projects/apps/forge && ./scripts/verify.sh
```

Paste the output block into Claude. Claude reads KEY + this file.

---

## SESSION KEY

Format: FG-<git-short-hash>-<YYYYMMDD>
Example: FG-abc1234-20260315

Claude protocol:
  1. Fetch this file from raw GitHub URL in the block
  2. Match commit hash to build status below
  3. Confirm: "Loaded FG-<hash>. Phase: <current>. Ready."
  4. Ask for task — never assume

---

## IDENTITY

Developer:  Harsh Maury
GitHub:     https://github.com/Harshmaury
Forge:      https://github.com/Harshmaury/Forge
Domain:     Execution — translates intent into platform actions
OS:         Ubuntu 24.04 (WSL2) + Windows 11

---

## PLATFORM CONTEXT

Forge is part of a three-domain developer platform:

  Control    Nexus   ~/workspace/projects/apps/nexus
  Knowledge  Atlas   ~/workspace/projects/apps/atlas
  Execution  Forge   ~/workspace/projects/apps/forge  ← this repo

Platform architecture:  ~/workspace/architecture/
ADRs:                   ~/workspace/architecture/decisions/

Forge port:  127.0.0.1:8082
Nexus port:  127.0.0.1:8080
Atlas port:  127.0.0.1:8081

Forge depends on Atlas Phase 1 being complete before
context enrichment works in the execution pipeline.

---

## MACHINE

Go:1.23.0  uuid v1.6.0  SQLite(Phase2)  yaml.v3(Phase2)  cobra

---

## BUILD STATUS

### ✅ Phase 1 — Command Execution (COMPLETE)
  internal/config/env.go              EnvOrDefault, ExpandHome
  internal/command/model.go           Command struct (ADR-004), RawCommandRequest, ExecutionResult
  internal/command/validator.go       5-field schema validation
  internal/command/validator_test.go  table-driven tests
  internal/command/translator.go       RawCommandRequest → validated Command
  internal/command/translator_test.go  translation rules
  internal/nexus/client.go              GetProject, GetAllProjects, Ping
  internal/atlas/client.go              GetProject, GetWorkspaceContext, Ping
  internal/context/resolver.go          ResolveContext, ValidateTarget — graceful degradation
  internal/context/resolver_test.go     mock clients, all enrichment + degradation cases, UUID generation, context defaults
  Requires: Atlas Phase 1 running
  cmd/forge/main.go             daemon entry point
  internal/command/model.go     Command struct (ADR-004 schema)
  internal/command/validator.go schema validation
  internal/command/translator.go raw input → Command object
  internal/executor/engine.go   execution pipeline
  internal/executor/intent/     build, test, run, deploy handlers
  internal/context/resolver.go  context from Atlas + Nexus
  internal/nexus/client.go      Nexus HTTP client
  internal/atlas/client.go      Atlas HTTP client
  internal/api/server.go        HTTP server on :8082

### ✅ Phase 2 — Workflow Definitions (COMPLETE)
### ✅ Phase 3 — Automation Triggers (COMPLETE)
  internal/store/storer.go   Trigger type, 5 interface methods
  internal/store/db.go       v2 migration — triggers table
  internal/trigger/model.go      CreateTriggerRequest, Filter, Matches()
  internal/trigger/registry.go   MatchingTriggers — load + evaluate
  internal/trigger/registry_test.go  filter + registry tests
  internal/store/storer.go   Workflow + WorkflowStep types, 7 interface methods
  internal/store/db.go       SQLite v1 — workflows + workflow_steps tables
  Requires: Phase 1 complete
  internal/store/db.go          SQLite workflow storage
  internal/workflow/             definition + execution

### ⏳ Phase 3 — Automation Triggers (NOT STARTED)
  Requires: Phase 2 complete
  internal/trigger/             event-to-workflow mapping

---

## CRITICAL FIXES

✅ FG-Fix-01  Unique command IDs per workflow run (uuid.New) (2026-03-16)
✅ FG-Fix-02  Bounded goroutine pool (semaphore, max 8) (2026-03-16)
✅ FG-Fix-03  build.go args: check before append (2026-03-16)
  internal/workflow/executor.go        uuid.New().String() for cmd.ID
  internal/trigger/subscriber.go       sem chan struct{} semaphore
  internal/executor/intent/build.go    args logic corrected

## FORGE CRITICALS — ALL COMPLETE ✅

## FORGE HIGHS

✅ FG-H-01  ResolveContext called once per workflow, not per step (2026-03-16)
✅ FG-H-02  SupportedEvents lookup type-safe nexusevents.Topic cast (2026-03-16)
✅ FG-H-03  WorkflowHandler.Create atomic via WithWorkflowTransaction (2026-03-16)
✅ FG-H-04  WorkflowHandler.Run correct 404/500 status codes (2026-03-16)
✅ FG-H-05  workspaceRoot from FORGE_WORKSPACE env var (2026-03-16)
✅ FG-H-06  SupportedEvents map keyed by nexusevents.Topic (2026-03-16)

## FORGE HIGHS — ALL COMPLETE ✅

## ADR-008 IMPLEMENTATION

✅ internal/nexus/client.go  WithServiceToken + get() helper
✅ internal/atlas/client.go  WithServiceToken + get() helper
✅ cmd/forge/main.go         FORGE_SERVICE_TOKEN env var

## DELIVERY PATTERN

Zip naming:  forge-<phase>-<what>-<YYYYMMDD>-<HHMM>.zip
Drop folder: /mnt/c/Users/harsh/Downloads/forge-drop/

Apply command:
  cd ~/workspace/projects/apps/forge && \
  unzip -o /mnt/c/Users/harsh/Downloads/forge-drop/<ZIP>.zip -d . && \
  go build ./... && \
  git add <files> WORKFLOW-SESSION.md && \
  git commit -m "<type>: <description>" && \
  git push origin <branch>

Full protocol: WORKFLOW-DELIVERY.md

---

## FORGE DESIGN RULES

- All input becomes a Command object before the executor sees it
- The executor never receives raw strings
- Service lifecycle goes through Nexus — Forge never calls providers directly
- Context enrichment queries Atlas — Forge never scans the filesystem
- Phase 2 does not start until Phase 1 command execution is proven
- uuid is generated by Forge if not supplied — id is never nil

---

## CHANGELOG

2026-03-16  v2.6.0  fix: FG-Fix-01+02+03 — unique cmd IDs, bounded goroutines, args fix
2026-03-16  v2.7.0  fix: FG-H-01~06 — resolve-once, type-safe events, atomic create, status codes, env, topic type
2026-03-16  v2.8.0  feat: ADR-008 — inter-service auth, outbound token on Nexus + Atlas clients
2026-03-15  v2.5.0  Phase 3 complete — trigger API, subscriber wired, smoke test
2026-03-15  v2.4.0  Phase 3 step 3 — trigger subscriber + tests
2026-03-15  v2.3.0  Phase 3 step 2 — trigger model, registry, filter matching + tests
2026-03-15  v2.2.0  Phase 3 step 1 — trigger store schema
2026-03-15  v2.1.0  Phase 2 complete — workflow API, store wired, smoke test
2026-03-15  v2.0.0  Phase 2 step 2 — workflow model + executor + tests
2026-03-15  v1.9.0  Phase 2 step 1 — workflow store schema
2026-03-15  v1.8.0  Phase 1 complete — main.go wired, all 8 steps done
2026-03-15  v1.7.0  Phase 1 step 7 — API handlers + server
2026-03-15  v1.6.0  Phase 1 step 6 — execution engine + tests
2026-03-15  v1.5.0  Phase 1 step 5 — intent handlers (build, test, run, deploy) + tests
2026-03-15  v1.4.0  Phase 1 step 4 — context resolver + tests
2026-03-15  v1.3.0  Phase 1 step 3 — Nexus + Atlas HTTP clients
2026-03-15  v1.2.0  Phase 1 step 2 — translator + tests
2026-03-15  v1.1.0  Phase 1 step 1 — config, command model, validator
2026-03-15  v1.0.0  Project scaffolded — documentation phase complete
