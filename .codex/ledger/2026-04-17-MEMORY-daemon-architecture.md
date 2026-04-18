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
  - Markdown artifacts remain part of the product model (`_prd.md`, `_techspec.md`, ADRs, task files, reviews, memory, protocol/prompt, QA outputs)
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
  - Current codebase still generates workflow/review `_meta.md` today; this is useful migration context but is superseded by the final daemon direction below.
  - Operational run state / audit artifacts:
    - `.compozy/runs/<run-id>/{run.json,result.json,events.jsonl,extensions.jsonl,jobs/,turns/}`
    - these are execution-state and audit-oriented, not product-planning documents
- Current design direction sharpened:
  - keep human-first workflow documents as real Markdown artifacts
  - `_tasks.md` and `_meta.md` do not survive in the daemonized model
  - move operational execution state behind the DB model
  - allow explicit sync/import of externally edited Markdown back into the DB
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
  - user approved the default transport stance for v1:
    - UDS for CLI
    - HTTP on localhost for local web/client surfaces
    - TCP disabled by default and only enabled through explicit configuration

State:

- Agora este turno está sendo usado como material de decomposição para o usuário: o pedido é um breakdown em slices implementáveis, com arquivos atuais tocados/criados, riscos de acoplamento e dependências naturais.

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
  - Early exploration considered a thinner file-backed shell over the current run artifacts, but that path was later superseded by the user-approved database-first model with `global.db` plus per-run `run.db`.
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
  - workflow `_meta.md` is not authored business content in the current codebase; it is refreshed from task counts (`tasks.RefreshTaskMeta`)
  - review round `_meta.md` is not authored review content in the current codebase; it is refreshed from issue status counts (`reviews.RefreshRoundMeta`)
  - review issues and task files are frontmatter-backed Markdown records with meaningful authored body content
  - memory files are explicitly modeled as Markdown documents with append/replace semantics and compaction signals
  - run artifacts are already treated as runtime metadata, logs, and event streams rather than workflow-planning docs
- Subagents were used for parallel exploration as requested, but the authoritative classification above is grounded in direct code/file inspection.
- Wrote the approved design document to `docs/plans/2026-04-17-compozy-daemon-design.md`.
- Ran a follow-up subagent audit against `~/.codex` history and tightened the AGH reference map around home layout, daemon boot, transport contracts, route parity, storage split, observer layering, and session manager lifecycle.
- Read the `cy-create-techspec` skill instructions plus the canonical `techspec-template.md` and `adr-template.md`.
- TechSpec exploration subagent mapped the highest-impact migration seams:
  - likely ownership boundaries already exist around `internal/cli`, `internal/core/kernel`, `internal/core/plan`, `internal/core/tasks`, `internal/core/reviews`, `internal/core/memory`, `internal/core/run/{executor,exec,ui,transcript}`, and `pkg/compozy/runs`
  - strongest migration pressure comes from current assumptions that `.compozy/runs` is rooted under the workspace via `internal/core/model/workspace_paths.go`, `internal/core/model/artifacts.go`, `internal/core/model/run_scope.go`, and `pkg/compozy/runs/{layout,run,watch}.go`
  - high-risk seams to avoid breaking blindly: `core <-> kernel` adapters, `extension/runtime` `init()` ownership around `RunScope`, public run reader layout coupling, and workspace discovery logic that currently derives identity from `cwd`
- Current code confirms the identity split in today's Compozy:
  - workflow identity is the user-facing workflow name/slug used for `.compozy/tasks/<name>` and CLI `--name` flags
  - run identity is a separate `run_id`, generated automatically in normal flows by `internal/core/model/run_scope.go` unless explicitly provided in runtime config
- User selected run identity option `B` for the TechSpec:
  - keep the current split between workflow slug and run id
  - daemon generates `run_id` by default in normal flows
  - explicit `run_id` remains allowed only for special flows such as replay/import/debug/advanced attach
- User selected the feature slug `daemon` for the TechSpec output path under `.compozy/tasks/daemon/`.
- Created accepted ADRs for the daemon effort under `.compozy/tasks/daemon/adrs/`:
  - `adr-001.md` global home-scoped singleton daemon
  - `adr-002.md` workspace Markdown plus home-scoped operational SQLite
  - `adr-003.md` AGH-aligned REST transports using Gin
  - `adr-004.md` TUI-first UX plus auto-start and explicit workspace operations
- Updated the design doc and ADR references after the `~/.codex` audit so `_tasks.md` / `_meta.md` are fully removed from the daemonized model and AGH reuse is anchored in exact files.
- Saved the approved TechSpec to `.compozy/tasks/daemon/_techspec.md` in English using the canonical template.
- Resolved the remaining CLI and transport detail gaps inside the TechSpec:
  - explicit workspace commands: `workspaces list|show|register|unregister|resolve`
  - explicit endpoint inventory for daemon, workspaces, tasks, reviews, runs, sync, and exec
  - run attach defaults under `[runs].default_attach_mode`
- Reran `make verify` after saving the TechSpec; formatting, lint, tests, and build all passed.
- Created `.codex/tmp/daemon-final-review-prompt.md` and ran:
  - `./bin/compozy exec --ide claude --reasoning-effort high --timeout 20m --persist --add-dir /Users/pedronauck/Dev/compozy/looper --add-dir /Users/pedronauck/dev/compozy/agh --prompt-file .codex/tmp/daemon-final-review-prompt.md`
- Captured the persisted review under run id `exec-20260417-181255-843268000`.
- Confirmed the Claude exec used multiple subagents and produced a final review centered on four highest-risk contract gaps:
  - startup reconciliation / post-crash recovery
  - `run.db` lifecycle and retention
  - explicit SSE contract copied from AGH semantics
  - `pkg/compozy/runs` migration contract
- Captured additional concrete follow-ups from the review:
  - UDS `0600`, daemon dir `0700`, explicit `127.0.0.1`, port persisted in `daemon.json`
  - `GET /runs/:run_id/snapshot`, `GET /daemon/health`, `GET /daemon/metrics`
  - extension transport clarified as stdio JSON-RPC in the run process plus per-run UDS capability token for Host API
  - request-id/error envelope, schema migration bookkeeping, artifact snapshot growth bounds, legacy `_meta.md` / `_tasks.md` cutover handling
- Updated `.compozy/tasks/daemon/_techspec.md` to incorporate the review:
  - startup reconciliation and explicit run lifecycle / retention / purge semantics
  - daemon-backed `pkg/compozy/runs` contract instead of direct SQLite reads
  - `GET /runs/:run_id/snapshot` plus explicit SSE contract (`Last-Event-ID`, heartbeat, overflow)
  - transport security and error envelope contract (`X-Request-Id`, `TransportError`, UDS `0600`, daemon dir `0700`, `127.0.0.1`)
  - `compozy exec` workspace binding semantics
  - legacy `_meta.md` / `_tasks.md` cleanup, watcher scope/debounce, archive conflict rules, and log rotation
- Reran `make verify` after the TechSpec update; formatting, lint, tests, and build all passed again.
- Identificados os pontos de encaixe atuais no código local para daemonização: `internal/cli/root.go`, `cmd/compozy/main.go`, `compozy.go`, `internal/core/model/{workspace_paths.go,run_scope.go,artifacts.go,runtime_config.go}`, `internal/core/run/{journal,event_stream}`, `internal/core/migration/migrate.go`, `pkg/compozy/runs/{layout,run,watch}.go`.
- Read the `cy-create-tasks` skill instructions and the canonical `task-template.md`.
- Confirmed `.compozy/config.toml` is absent, so the default task types apply: `frontend`, `backend`, `docs`, `test`, `infra`, `refactor`, `chore`, `bugfix`.
- Presented a 16-task daemon breakdown to the user, covering daemon bootstrap, storage, transport, run manager, sync/watch/archive, extension runtime, CLI/TUI, public run readers, reviews/exec migration, and final cleanup.
- User approved the breakdown (`pode dale`).
- Wrote `.compozy/tasks/daemon/_tasks.md` plus enriched `task_01.md` through `task_16.md`.
- Used two explorer subagents to validate the remaining task-file enrichment against real code seams:
  - runtime/CLI/TUI/public-reader paths
  - sync/archive/watcher paths
- Incorporated those explorer findings into the final task-file relevant files, dependent files, requirements, and test guidance before validation.
- Ran `./bin/compozy validate-tasks --name daemon` and confirmed `all tasks valid (16 scanned)`.
- Ran `make verify` after task generation; formatting, lint, tests, and build all passed, with `DONE 1940 tests` and `All verification checks passed`.
- User asked for the greenfield-vs-compatibility posture to be made explicit; updated the TechSpec with `Compatibility Posture` and added stronger anti-compat-shim requirements to tasks `11`, `13`, `14`, and `16`.
- User then asked for explicit AGH file references; updated the TechSpec with an `AGH Reference Map` section and added `### AGH Reference Files` subsections to all daemon task files.
- Re-ran `./bin/compozy validate-tasks --name daemon`; it still passed with `all tasks valid (16 scanned)`.
- Re-ran `make verify` after the documentation/task updates. It no longer fails on the earlier missing `go.sum` hint after `go get github.com/charmbracelet/ultraviolet@v0.0.0-20260205113103-524a6607adb8`, but it is currently blocked by unrelated lint failures in:
  - `internal/store/globaldb/migrations.go` (`tx.Rollback` errcheck)
  - `internal/store/globaldb/registry.go` (`tx.Rollback` errcheck)
  - `internal/store/sqlite.go` (blank import justification)
- User judged the generated task test matrices too weak and asked to strengthen tests for every daemon task and reset all tasks to `pending` so the feature can be restarted cleanly.
- Updated `.compozy/tasks/daemon/_tasks.md` to reset task `01` from `completed` to `pending`, leaving the whole daemon backlog uniformly pending again.
- Rewrote the `## Tests` sections in `.compozy/tasks/daemon/task_01.md` through `.compozy/tasks/daemon/task_16.md` to add broader coverage for:
  - crash/restart and reconciliation
  - concurrency and duplicate-run conflicts
  - daemon bootstrap edge cases
  - sync/watch/archive error paths
  - extension runtime failures
  - CLI/TUI attach/reconnect behavior
  - public run-reader compatibility
  - final end-to-end regression and docs cleanup
- Re-ran `./bin/compozy validate-tasks --name daemon`; it passed again with `all tasks valid (16 scanned)`.
- Re-ran `make verify` after the task rewrites; it is still blocked by the same unrelated lint failures in `internal/store/globaldb/{migrations.go,registry.go}` and `internal/store/sqlite.go`.

Now:

- Responder ao usuário com o resumo das task updates, including the fact that the backlog is reset to `pending` and the current repo verification is still blocked by unrelated lint issues.

Next:

- Se o usuário quiser, tratar separadamente os lint failures atuais do pacote `internal/store`.

Open questions (UNCONFIRMED if needed):

- None blocking.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-17-MEMORY-daemon-architecture.md`
- `.codex/tmp/daemon-final-review-prompt.md`
- `.compozy/runs/exec-20260417-181255-843268000/run.json`
- `.compozy/runs/exec-20260417-181255-843268000/events.jsonl`
- `.compozy/runs/exec-20260417-181255-843268000/turns/0001/stdout.log`
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
- `/Users/pedronauck/dev/compozy/agh/internal/api/core/handlers.go`
- `/Users/pedronauck/dev/compozy/agh/internal/api/core/sse.go`
- `/Users/pedronauck/dev/compozy/agh/internal/api/httpapi/server.go`
- `/Users/pedronauck/dev/compozy/agh/internal/api/httpapi/routes.go`
- `/Users/pedronauck/dev/compozy/agh/internal/api/udsapi/server.go`
- `/Users/pedronauck/dev/compozy/agh/internal/api/udsapi/routes.go`
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
- `.compozy/tasks/daemon/_tasks.md`
- `.compozy/tasks/daemon/task_01.md`
- `.compozy/tasks/daemon/task_02.md`
- `.compozy/tasks/daemon/task_03.md`
- `.compozy/tasks/daemon/task_04.md`
- `.compozy/tasks/daemon/task_05.md`
- `.compozy/tasks/daemon/task_06.md`
- `.compozy/tasks/daemon/task_07.md`
- `.compozy/tasks/daemon/task_08.md`
- `.compozy/tasks/daemon/task_09.md`
- `.compozy/tasks/daemon/task_10.md`
- `.compozy/tasks/daemon/task_11.md`
- `.compozy/tasks/daemon/task_12.md`
- `.compozy/tasks/daemon/task_13.md`
- `.compozy/tasks/daemon/task_14.md`
- `.compozy/tasks/daemon/task_15.md`
- `.compozy/tasks/daemon/task_16.md`
- Commands: `rg`, `sed`, `find`, `git log`
