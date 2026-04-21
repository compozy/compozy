Goal (incl. success criteria):

- Create the task breakdown for `.compozy/tasks/daemon-improvs/analysis` from the approved `_techspec.md`, generate `_tasks.md` plus enriched `task_NN.md` files, and validate them with `compozy tasks validate`.
- Success means: load the saved TechSpec and ADRs, derive independently implementable tasks with explicit dependencies, get user approval on the breakdown, write all task artifacts, and finish with a passing task validation run.

Constraints/Assumptions:

- Must follow the `cy-create-tasks` skill and the repo instructions in `AGENTS.md` / `CLAUDE.md`.
- Default collaboration mode does not expose the blocking interactive question tool, so any required approval step must be handled as a normal assistant message.
- The task root is assumed to be `.compozy/tasks/daemon-improvs/analysis` because that is where the approved `_techspec.md` and ADRs were saved.
- `_prd.md` is absent; task decomposition will be TechSpec-driven rather than PRD-driven.
- `.compozy/config.toml` does not exist, so allowed task types fall back to the built-in defaults: `frontend`, `backend`, `docs`, `test`, `infra`, `refactor`, `chore`, `bugfix`.
- Read-only awareness of other `.codex/ledger/*-MEMORY-*.md` files is required; never modify another session ledger.
- User preference: when presenting multiple-choice recommendations, always append `(recomendado)` to the recommended option.

Key decisions:

- Use the approved TechSpec at `.compozy/tasks/daemon-improvs/analysis/_techspec.md` as the primary decomposition source.
- Use the ADRs under `.compozy/tasks/daemon-improvs/analysis/adrs/` to constrain task boundaries and dependency order.
- Use local codebase exploration rather than delegated sub-agents because this run has no explicit delegation request.
- Derive tasks for the nested workflow directory `daemon-improvs/analysis` instead of the parent `daemon-improvs` directory.
- User selected scope `C`:
  - keep the current daemon architecture as the baseline
  - allow one targeted structural refactor where the analysis shows the current boundary is wrong
- User wants the TechSpec to cover both:
  - `A` transport/API contracts
  - `C` runtime supervision/lifecycle hardening
- User selected `A` as the structural refactor boundary and `C` as the primary hardening track:
  - `A` = contracts/DTOs/SSE/HTTP/UDS/client boundaries get the deeper structural cleanup
  - `C` = shutdown, signals, orphan reaping, process-group kill, checkpoint, and recovery stay as hardening within the current daemon architecture
- User selected the broad contract-boundary option:
  - create `internal/api/contract` as the canonical daemon contract
  - converge handlers, daemon client, SSE, and `pkg/compozy/runs` around that shape
  - use adapters only where public API stability requires them
- User selected the highest validation bar:
  - parity tests for HTTP/UDS are part of the primary scope
  - a reusable runtime harness is part of the primary scope
  - ACP fault-injection coverage is part of the primary scope
- User selected observability as part of the same primary scope:
  - richer `/daemon/health`
  - richer `/daemon/metrics`
  - strong run snapshot contract
  - canonical transcript assembly

State:

- TechSpec creation is complete.
- Task decomposition is complete and validated.
- QA follow-up tasks were added to the workflow and the updated task set is validated.

Done:

- Read repository instructions from `AGENTS.md`.
- Read the `cy-create-techspec` skill instructions and canonical `techspec-template.md` / `adr-template.md`.
- Read the `brainstorming` skill instructions because this is design work.
- Scanned hidden repo artifacts and relevant prior daemon ledgers.
- Read `.compozy/tasks/daemon-improvs/analysis/analysis.md`.
- Read `.codex/ledger/2026-04-17-MEMORY-daemon-architecture.md`.
- Read `.codex/ledger/2026-04-17-MEMORY-daemon-run-manager.md`.
- Read `.compozy/tasks/daemon/_techspec.md` for prior daemon architecture context.
- Read the daemon-improvement analysis pack to identify the main decision seams: resilience bundle, task-runtime recovery, transport contracts, observability, and test harness maturity.
- Read the live daemon runtime code in `internal/daemon/{host.go,boot.go,reconcile.go,service.go}`, `internal/api/client/client.go`, `internal/api/core/interfaces.go`, `internal/store/{globaldb/global_db.go,rundb/run_db.go}`, and `internal/core/subprocess/process.go`.
- Confirmed the highest-impact improvement seams still match the analysis pack:
  - daemon shutdown still uses `context.Background()` in `closeHostRuntime`
  - daemon health still reports a minimal readiness payload
  - transport DTOs still live inline in `internal/api/core/interfaces.go`
  - daemon client still applies a blanket `5s` request timeout
  - `GlobalDB.Close()` / `RunDB.Close()` still close raw SQLite handles without explicit checkpoint orchestration
  - subprocess shutdown still targets the managed pid rather than a process group
- Asked and resolved the technical-clarification sequence:
  - hybrid scope
  - `A` contracts/API + `C` runtime hardening
  - `A` as the structural refactor boundary
  - canonical `internal/api/contract` used by handlers, client, SSE, and `pkg/compozy/runs`
  - parity tests, runtime harness, and ACP fault injection in primary scope
  - observability treated as first-class in the same spec
- Created ADRs under `.compozy/tasks/daemon-improvs/analysis/adrs/`:
  - `adr-001.md` canonical daemon transport contract
  - `adr-002.md` incremental runtime supervision hardening inside the existing daemon boundary
  - `adr-003.md` validation-first daemon hardening
  - `adr-004.md` observability as a first-class daemon contract
- Ran an external spec review through Compozy as requested:
  - command: `./bin/compozy exec --ide claude --model opus --reasoning-effort xhigh --format json --add-dir /Users/pedronauck/Dev/compozy/looper --prompt-file .codex/tmp/daemon-improvs-techspec-review.md`
  - run id: `exec-20260421-003740-000000000`
  - review result: `approve_with_edits`
  - main blocking findings:
    - the draft `RunSnapshot` sample diverged from the current canonical shape
    - API endpoint inventory was incomplete versus `internal/api/core/routes.go`
    - development sequencing put hardening ahead of the real harness/fault tooling needed to validate it
    - client timeout policy lacked concrete defaults and composition rules
    - the illustrative `RunSupervisor` interface did not clearly map onto the existing service split
    - ACP stall/recovery ownership was under-specified
    - Windows parity for signals/process groups was hand-waved
  - main non-blocking findings:
    - metrics schema needs explicit type/label/unit rules
    - error-code vocabulary should be frozen in the contract
    - daemon log rotation and mirroring policy should be explicit
    - CLI parity test scope should be enumerated
    - build-tag / integration-lane strategy should be explicit
    - snapshot `Incomplete` semantics should be formally defined
    - impact analysis should include `cmd/compozy`, `internal/core/run/executor`, and `internal/logger`
- Revised the TechSpec draft in `.codex/tmp/daemon-improvs-techspec-draft-v2.md` to address the external review findings:
  - aligned `RunSnapshot` with the current canonical shape and made `Incomplete` additive/sticky
  - enumerated the full current route inventory from `internal/api/core/routes.go`
  - reordered development sequencing so harness and parity infrastructure land before the riskiest hardening work
  - defined concrete client timeout classes and stream heartbeat/reconnect rules
  - clarified that the new `RunSupervisor` facade is daemon-internal and does not replace the existing transport-facing service split
  - assigned ACP stall/recovery ownership to `internal/core/run/executor` via liveness monitoring
  - made the Windows stance explicit: Unix-only process-group/orphan handling in this phase, compile-safe Windows behavior, no implied parity claim
  - added explicit metrics schema, log rotation policy, integration build tags, and CLI parity scope
- Saved the approved TechSpec to `.compozy/tasks/daemon-improvs/analysis/_techspec.md`.
- Ran `make verify` after saving the artifact and it passed:
  - `fmt`: passed
  - `lint`: passed with `0 issues`
  - `test`: passed with `DONE 2438 tests, 1 skipped`
  - `build`: passed and rebuilt `bin/compozy`
- Started `cy-create-tasks` for the same feature directory.
- Loaded the task template and frontmatter schema from `.agents/skills/cy-create-tasks/references/`.
- Confirmed `.compozy/config.toml` is absent; using default task types.
- Confirmed the feature directory currently contains `_techspec.md`, analysis source docs, and ADRs, but no `_tasks.md` or `task_*.md` files yet.
- Re-read the saved TechSpec and ADRs to drive the task breakdown.
- Re-inspected the main affected code areas:
  - `internal/api/core`, `internal/api/client`, `internal/api/httpapi`, `internal/api/udsapi`
  - `internal/daemon`, `internal/core/subprocess`, `internal/core/run/executor`
  - `internal/store/globaldb`, `internal/store/rundb`, `pkg/compozy/runs`
  - `internal/cli`, `cmd/compozy`
- Confirmed planned new packages from the TechSpec do not exist yet:
  - `internal/api/contract`
  - `internal/logger`
  - `internal/testutil/e2e`
  - `internal/testutil/acpmock`
- Generated the task artifacts under `.compozy/tasks/daemon-improvs/analysis/`:
  - `_tasks.md`
  - `task_01.md` through `task_07.md`
- Ran `./bin/compozy tasks validate --tasks-dir .compozy/tasks/daemon-improvs/analysis` and it passed with `all tasks valid (7 scanned)`.
- Ran `make verify` after writing the task files and it passed:
  - `fmt`: passed
  - `lint`: passed with `0 issues`
  - `test`: passed with `DONE 2438 tests, 1 skipped`
  - `build`: passed and rebuilt `bin/compozy`
- Added the missing QA follow-up tasks to mirror the daemon workflow:
  - `task_08.md` = `/qa-report` planning and regression artifacts
  - `task_09.md` = `/qa-execution` validation and operator-flow execution
- Updated `.compozy/tasks/daemon-improvs/analysis/_tasks.md` to include tasks `08` and `09`.
- Re-ran `./bin/compozy tasks validate --tasks-dir .compozy/tasks/daemon-improvs/analysis` and it passed with `all tasks valid (9 scanned)`.
- Re-ran `make verify` after adding the QA tasks and it passed:
  - `fmt`: passed
  - `lint`: passed with `0 issues`
  - `test`: passed with `DONE 2438 tests, 1 skipped`
  - `build`: passed and rebuilt `bin/compozy`

Now:

- Task generation is complete; report artifact paths and verification evidence.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED: final save path is assumed to be `.compozy/tasks/daemon-improvs/analysis/_techspec.md` because the user invoked `cy-create-techspec` with that path.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-20-MEMORY-daemon-improvs-techspec.md`
- `.compozy/tasks/daemon-improvs/analysis/_techspec.md`
- `.compozy/tasks/daemon-improvs/analysis/_tasks.md`
- `.compozy/tasks/daemon-improvs/analysis/task_{01..09}.md`
- `.compozy/tasks/daemon-improvs/analysis/adrs/{adr-001.md,adr-002.md,adr-003.md,adr-004.md}`
- `.compozy/tasks/daemon-improvs/analysis/{analysis.md,analysis_core_lifecycle.md,analysis_resources_reconcile.md,analysis_transport_api.md,analysis_task_runtime.md,analysis_observability_storage.md,analysis_testing_harness.md}`
- `.compozy/tasks/daemon/_techspec.md`
- `.codex/ledger/2026-04-17-MEMORY-daemon-architecture.md`
- `.codex/ledger/2026-04-17-MEMORY-daemon-run-manager.md`
- `.agents/skills/cy-create-tasks/references/{task-template.md,task-context-schema.md}`
- Commands: `rg`, `sed`, `find`, `./bin/compozy tasks validate --help`
