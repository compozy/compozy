Goal (incl. success criteria):

- Brainstorm how to evolve Compozy from per-execution CLI runtime into a single-binary background daemon, informed by the existing AGH daemon architecture, and reach an approved design direction that can be turned into a TechSpec next.
- Success means: clear problem framing, explicit trade-offs, one recommended architecture direction, and user-approved design guidance for the following `cy-create-techspec` step.

Constraints/Assumptions:

- This turn is design work only under the `brainstorming` skill. Do not implement code yet.
- The user explicitly wants subagent analysis in both repositories: this repo and `/Users/pedronauck/dev/compozy/agh`.
- Runtime is in Default collaboration mode; the blocking interactive question tool is unavailable here, so any brainstorming question must be asked as the complete assistant message and execution must pause.
- Read-only awareness of other `.codex/ledger/*MEMORY*.md` files is required; never modify another session ledger.
- User prefers discussion in pt-BR from this point onward.

Key decisions:

- Use AGH as a first-party reference and reuse candidate components/patterns aggressively where that reduces design risk.
- Keep the scope greenfield-friendly; backward-compatibility constraints are intentionally weak unless they protect operator ergonomics.
- Initial working hypothesis: Compozy already has strong run orchestration, extension hooks, and persisted run artifacts, so the transition likely centers on adding a durable host process and transport surfaces rather than replacing the execution engine itself.
- Confirmed through direct reads plus subagents: the recommended direction is to wrap Compozy’s existing run engine (`RunScope`, journal, event bus, kernel commands, public run readers) in a daemon shell, while borrowing AGH’s composition-root, boot sequencing, singleton lock, and shared transport-contract patterns.
- Explicitly avoid porting AGH’s full global/session SQLite model and larger resource kernel into Compozy v1 daemon mode unless a later design decision proves the current file+journal persistence insufficient.
- User clarified the primary product priority for the daemonized Compozy:
  - combine `API/Web-first` with `extensibility-first`
  - build a richer local platform from the start
  - preserve the current Compozy model/capabilities
  - do not introduce AGH-style top-level `automation` as a new separate concept in v1
- User selected extension lifecycle option `A`:
  - the daemon is long-lived
  - extensions remain run-scoped subprocesses
  - extensions may still interact with the daemon API/control plane, but they do not become resident services across runs
- User selected a database-first persistence model for v1:
  - SQLite becomes the primary source of truth for daemon state
  - Markdown artifacts remain part of the product model (`_prd.md`, `_techspec.md`, `_tasks.md`, task files, reviews, etc.)
  - workflow execution still starts from flexible artifact generation outside the daemon
  - when `start` / review execution runs, Compozy parses the artifacts and synchronizes them into the database
  - event persistence may move from `events.jsonl` toward SQLite in a shape closer to AGH
  - AGH’s database split/schema strategy is now an explicit reference candidate
- User selected SQLite topology option `A`:
  - two SQLite databases, mirroring AGH at a high level
  - one global DB for catalog and operational state
  - one DB per run for detailed events, transcript, and event streaming
- Artifact classification from current Compozy codebase:
  - Human-first workflow documents:
    - `_prd.md`, `_techspec.md`, ADRs, `task_XX.md`, `reviews-NNN/issue_XXX.md`, `memory/{MEMORY.md,task_XX.md}`, `_protocol.md`, `_prompt.md`, `qa/` outputs
  - Derived/projection-style workflow metadata:
    - `_tasks.md` should join the derived/projection group instead of remaining a first-class authored document
    - workflow `_meta.md` under `.compozy/tasks/<name>/` is generated from task counts/status and refreshed by `compozy sync`
    - review round `_meta.md` under `reviews-NNN/` is derived from round identity + issue resolution counts and refreshed when review status changes
  - Operational run state / audit artifacts:
    - `.compozy/runs/<run-id>/{run.json,result.json,events.jsonl,extensions.jsonl,jobs/,turns/}`
    - these are execution-state and audit-oriented, not product-planning documents
- Current design direction sharpened:
  - keep human-first workflow documents as real Markdown artifacts
  - move derived metadata and operational execution state behind the DB model
  - allow explicit sync/import of externally edited Markdown back into the DB
  - `_tasks.md` is now treated like `_meta.md`: a projection/materialization from DB state rather than an authored source document
  - reconciliation model is hybrid and performance-aware:
    - `compozy sync` remains an explicit reconciliation command
    - `start` and `fix-reviews` perform reconciliation automatically before execution
    - active runs may attach scoped file watchers for their own workflow/run context
    - broad always-hot whole-workspace watching is explicitly avoided
- contract direction is layered, not JSON-RPC everywhere:
  - in-process daemon services should communicate via typed Go interfaces/direct calls
  - operator-facing daemon API should likely use shared HTTP/JSON contracts plus SSE, exposed over both UDS and TCP
  - subprocess/plugin boundaries should keep JSON-RPC where it already fits (extensions)
  - agent runtime communication should keep ACP as its own protocol boundary
  - user approved this contract split

State:

- In progress; clarification is sufficient and design presentation has started.

Done:

- Read repository-level instructions from `AGENTS.md` and `CLAUDE.md` in Compozy.
- Read the `brainstorming` skill instructions.
- Scanned existing ledgers for prior design context relevant to agents, extensibility, early run scope, and headless workflow execution.
- Read Compozy README and extensibility architecture docs for current execution/runtime behavior.
- Collected recent Compozy commits and searched for runtime/daemon/headless/extension references.
- Read AGH repository instructions and mapped its daemon-oriented package layout at a high level.
- Collected AGH daemon/runtime package hits showing composition root, HTTP/SSE API, and UDS CLI transport.
- Read AGH daemon composition files covering staged boot, runtime dependency injection, lock/info/orphan handling, server factories, resource reconcile, extensions, hooks, network, automation, and shutdown flow.
- Read AGH API contract/core transport files covering shared DTOs, shared handler/service interfaces, SSE helpers, error/status mapping, HTTP server wiring, UDS server wiring, and transport-specific prompt/extension differences.
- Read key AGH session files covering manager construction, session model, hook domains, and environment lifecycle orchestration.
- Read AGH persistence and observability files covering the global DB schema, per-session event DB, and observer/health query surfaces.
- Spawned explorer subagents for Compozy runtime analysis, AGH daemon analysis, and cross-repo reuse mapping.
- Received completed explorer findings confirming:
  - Compozy’s strongest daemon-ready seam is the existing `RunScope` plus durable journal/event bus and `pkg/compozy/runs` readers/watchers.
  - AGH’s most portable patterns are staged daemon boot, dual transport with shared contracts, singleton lock/readiness info, and targeted reconciliation/failover ideas.
  - The smallest viable target is a local singleton `compozyd` with UDS-first and HTTP-second control surfaces, backed by Compozy’s current run artifacts rather than a new database-first model.
- Closed all subagents after the relevant summaries were captured.
- Inspected the current filesystem artifact tree under `.compozy/tasks` and `.compozy/runs`.
- Read the main artifact ownership code paths:
  - `internal/core/tasks/{store.go,validate.go}`
  - `internal/core/reviews/{store.go,parser.go}`
  - `internal/core/{sync.go,archive.go,fetch.go}`
  - `internal/core/migration/migrate.go`
  - `internal/core/memory/store.go`
  - `internal/core/run/{executor/result.go,exec/exec.go}`
  - `internal/core/model/{task_review.go,workspace_paths.go,artifacts.go}`
  - `pkg/compozy/runs/layout/layout.go`
- Confirmed concrete artifact roles from code:
  - workflow `_meta.md` is not authored business content; it is refreshed from task counts (`tasks.RefreshTaskMeta`)
  - review round `_meta.md` is not authored review content; it is refreshed from issue status counts (`reviews.RefreshRoundMeta`)
  - review issues and task files are frontmatter-backed Markdown records with meaningful authored body content
  - memory files are explicitly modeled as Markdown documents with append/replace semantics and compaction signals
  - run artifacts are already treated as runtime metadata, logs, and event streams rather than workflow-planning docs
- Subagents were used for parallel exploration as requested, but the authoritative classification above is grounded in direct code/file inspection.

Now:

- Present the proposed architecture/design sections for user validation, including the CLI/daemon execution model.

Next:

- Use the user’s answer to frame 2-3 architecture approaches with trade-offs and a recommendation.
- After brainstorming approval, write the design doc and move into `cy-create-techspec`.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED: Should the first daemonized Compozy release primarily optimize for local CLI compatibility, for a web/API control plane, or for extension-driven automation as the top priority?
- UNCONFIRMED: Whether Compozy should embed a persistent database like AGH or continue leaning on run-artifact storage for v1 daemon mode; current evidence favors reusing run artifacts first.
- Extensions should remain run-scoped subprocesses in v1 daemon mode.
- SQLite is the primary source of truth in v1 daemon mode.
- Markdown workflow artifacts remain durable product artifacts and must sync into the database during execution flows.
- Compozy should use two SQLite databases in v1: one global DB and one per-run events DB.
- `_tasks.md` should be treated as a derived view/materialization, not a human-authored source document.
- Human-first workflow documents should reconcile through explicit sync plus targeted run-scoped watching rather than full-time global filesystem reconciliation.
- UNCONFIRMED: final operator-facing daemon transport shape, but current recommendation is shared HTTP/JSON + SSE contracts over UDS/TCP instead of universal JSON-RPC.
- Recommended operator-facing transport split has been validated conceptually by the user.
- User correctly identified one remaining design gap: the CLI execution model and daemon lifecycle from the operator perspective still need to be specified explicitly.
- User challenged the proposed CLI resource name `workflows`; this term may be too abstract/confusing for the daemonized CLI and should be reconsidered before freezing the command taxonomy.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-17-MEMORY-daemon-architecture.md`
- `README.md`
- `docs/extensibility/architecture.md`
- `.codex/ledger/2026-04-10-MEMORY-compozy-agents-techspec.md`
- `.codex/ledger/2026-04-10-MEMORY-extension-lifecycle.md`
- `.codex/ledger/2026-04-10-MEMORY-early-run-scope.md`
- `.codex/ledger/2026-04-11-MEMORY-workflow-headless.md`
- `/Users/pedronauck/dev/compozy/agh/AGENTS.md`
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/boot.go`
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/daemon.go`
- `/Users/pedronauck/dev/compozy/agh/internal/api/core/interfaces.go`
- `/Users/pedronauck/dev/compozy/agh/internal/api/httpapi/server.go`
- `/Users/pedronauck/dev/compozy/agh/internal/api/udsapi/server.go`
- `/Users/pedronauck/dev/compozy/agh/internal/session/manager.go`
- `/Users/pedronauck/dev/compozy/agh/internal/store/globaldb/global_db.go`
- `/Users/pedronauck/dev/compozy/agh/internal/store/sessiondb/session_db.go`
- `/Users/pedronauck/dev/compozy/agh/internal/observe/observer.go`
- `internal/core/model/run_scope.go`
- `internal/core/extension/runtime.go`
- `internal/core/run/journal/journal.go`
- `internal/core/kernel/handlers.go`
- `internal/core/plan/prepare.go`
- `internal/core/run/executor/execution.go`
- `pkg/compozy/runs/run.go`
- `pkg/compozy/runs/watch.go`
- Commands: `rg`, `sed`, `find`, `git log`
