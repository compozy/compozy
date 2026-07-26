# L-026 — Integration claims require substrate evidence, not symbol greps

**Class:** Analysis / Architecture review
**Date discovered:** 2026-07-07
**Evidence sources:** Loops × task-domain architecture session (2026-07-07); coupling audit of `internal/loop`; archived loops spec ADRs.

## Context

During the loops × task-domain review, an initial architectural critique claimed that `internal/loop` "does not talk to the native task domain at all" because a grep for `CreateTask` inside `internal/loop` returned nothing. The claim was wrong at the root: the bridge IS the core architecture. Loop coordinators and action nodes are persisted and executed as `task_runs` (`run_kind='coordinator'|'worker'` + `loop_run_id`), reserved through `EnqueueRun`/coordinator completion plans and claimed via `task.Service.ClaimNextRun` — exactly as the owning ADR decided: *"a Loop node becomes an AGH task; a node→node edge becomes an AGH dependency (kind `blocks`); a node's execution is an AGH run"* (archived ADR-002). The same session also "recommended" task operations as tool actions rather than bespoke node kinds — something ADR-021 had already decided verbatim ("`agh__task_*` already cover the premise").

## Root cause

Absence of one API symbol was treated as absence of integration. Cross-domain bridges live in schema columns, discriminator enums, store wiring, and injected seams — not necessarily in the symbol you guessed. Compounding it, the critique was delivered before reading the owning spec/ADRs, so it re-litigated (and partially re-invented) decisions that were already written down.

## Rule

> Before claiming "X does not integrate with Y" — or critiquing an architectural decision — verify three surfaces: (1) the schema (correlation columns, FKs, partial indexes, migrations), (2) the domain's enums/discriminators (e.g. `RunKind`), (3) the owning ADR/techspec (including `.compozy/tasks/_archived/`). An architectural verdict issued before reading the owning spec is invalid.

## Operationalization

- Grep the DDL + migration registry for correlation columns before concluding two domains are disconnected (`ALTER TABLE ... ADD COLUMN <domain>_id` is the classic bridge shape).
- Grep type enums for discriminators (`RunKind`, `origin_kind`, `owner_kind`) — bridges often ride an existing entity with a kind field instead of a new call path.
- Locate the owning spec first: `.compozy/tasks/_archived/<slug>/adrs/` and `docs/design/` hold the decision record even after the task tree is archived.

## Anti-pattern

- Single-symbol grep verdicts: "no `CreateTask` call in the package → no integration".
- Presenting as a novel recommendation something an ADR already decided (or already rejected).
- Judging integration by API-call topology while ignoring that the integration deliberately flows through a shared substrate (here: `task_runs` as the single durable work queue, L-003).

## Source

- `internal/store/globaldb/global_db.go:301` — partial unique index on `task_runs(loop_run_id)`; `:1408-1412` — migrations adding `run_kind` + `loop_run_id` to `task_runs`.
- `internal/loop/control_readiness.go:222-234` — node executions enqueued as `task.EnqueueSpec{RunKind: RunKindWorker, LoopRunID: ...}`.
- `internal/task/types.go:161-183` — `RunKind` enum (`worker | coordinator`).
- `.compozy/tasks/_archived/1783422111817-2d088f48-loops/adrs/adr-002.md` (node = task, execution = run) and `adr-021.md` (tools, not bespoke kinds).
