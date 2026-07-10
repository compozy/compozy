---
type: bug-fix
created: 2026-07-09 16:24
source: compozy/compozy · main · feature landed via PR #200 (6fa1d4b), fixes #217 (cc626ba), #223 (1219df3); related #216 (73ced7f COMPOZY_HOME)
---

# Worktrees + parallel tasks: broken selection UX, post-wave stalls, unwanted worktree creation, missing per-run worktree choice, and cross-layer inconsistencies

## Problem

Users report the worktrees + parallel-tasks feature is "cheia de bugs". Verbatim report:

> usuários estão reclamando que a nova feature de worktrees e parallel tasks estão cheias de bugs: seleção na TUI acontecendo errado, conforme move as setas algumas tasks não são selecionadas, ao finalizar uma wave eles dizem que o processo é parado e mais nada acontece, worktrees são criadas a todo momento mesmo rodando uma única spec e não multiplas, o wizard não tem a opção de caso rode multiplas specs rodar elas em worktree ou não, inconsistencia e problemas em geral

Decomposed into five bug clusters (labels used throughout this brief):

- **B1 — TUI selection:** moving with arrow keys, some tasks end up not selected. Expected: what the user believes they selected is exactly what runs.
- **B2 — post-wave stall:** after a wave finishes, "the process stops and nothing else happens". Expected: a run always progresses to a user-visible terminal state, and the observing CLI/TUI reflects it.
- **B3 — worktrees for a single spec:** worktrees are created "all the time" even when running one spec, not multiple. Expected: worktree-backed execution happens only when the user chose it, and its disk footprint is bounded and cleaned.
- **B4 — missing wizard option:** the wizard offers no choice, when running multiple specs, to run them in worktrees or not. Expected: an explicit, discoverable per-run choice.
- **B5 — general inconsistency:** behaviors, events, statuses, cleanup, and quit semantics differ across code paths that users perceive as one feature.

Severity: user-facing, affects the flagship parallel-execution flow (`compozy tasks run`), causes hung CLI sessions (perceived data-loss/abandonment) and unbounded disk growth under `~/.compozy/state/worktrees`.

## Current State

Findings below come from a 6-slice static exploration; every load-bearing citation was sample-verified against source on `main`. Full analyses: `.compozy/tasks/worktree-parallel-bugs/analysis/` (see References).

### 1. Run-start routing (daemon) — two "parallel" products, one auto-upgrade

- `StartTaskRun` (single spec) always consults `startParallelTaskRunIfEnabled` first (`internal/daemon/run_manager.go:421-459`, call at `:437-448`). That function routes the run into worktree-backed per-task parallel execution purely from config:

  ```go
  parallelCfg = parallelCfg.ApplyDefaults()
  if parallelCfg.Enabled == nil || !*parallelCfg.Enabled {
      return apicore.Run{}, false, nil
  }
  ```

  (`internal/daemon/task_multi.go:121-124`; on `true` it builds a per-task wave plan for that one slug, forces `mode=parallel`, and starts a `task_multi` parent — `:125-179`.)

- `Enabled` resolves from workspace TOML `[tasks.run.parallel] enabled` merged with CLI `--parallel-tasks` overrides (`internal/daemon/run_manager.go:2658-2681`; default `false` in `internal/core/workspace/config_types.go:24-26,102-128`). Once it resolves `true` there is no per-run opt-out on the daemon side.
- Explicit multi-spec (`StartTaskRunMultiple`, `internal/daemon/task_multi.go:74-106`) never sets `parallelTasks` and ignores `ParallelTasksConfig` (test `internal/daemon/task_multi_test.go:3013-3046`). Its mode is `enqueued` (serial, no worktrees) or `parallel` (one worktree per slug, no integration branch) via `resolveTaskMultiMode` (`:490-503`).
- Decision matrix (verified in slice 03): single slug + `parallel_tasks.enabled=true` ⇒ per-task worktrees + integration worktree; multi-slug `mode=parallel` ⇒ per-slug worktrees; multi-slug enqueued ⇒ no worktrees; CLI rejects `--parallel-tasks` together with `--multiple` (`internal/cli/daemon_commands.go:816-819`).
- The parent coordinator dispatches three different execution paths with different event families, failure policies (fail-fast enqueued vs fail-late parallel), and recovery policies (within-spec children get recovery force-disabled at `internal/daemon/task_multi.go:1700`; multi-slug children keep it) (`:553-577`, `:1187-1374`).

### 2. Waiting on children (daemon) — unbounded poll

All three paths wait on child runs via `waitForTaskMultiChild` (`internal/daemon/task_multi.go:1828-1849`), a 100ms DB poll (`taskMultiChildPollInterval`, `:36`) that exits only on a terminal child status or parent ctx cancel:

```go
for {
    row, err := m.globalDB.GetRun(detachContext(ctx), trimmedRunID)
    ...
    if isTerminalRunStatus(row.Status) {
        return row, nil
    }
    select {
    case <-ctx.Done():
        ...
    case <-ticker.C:
    }
}
```

There is no timeout or liveness check; a child process that dies without persisting a terminal status blocks the parent (and the whole wave) indefinitely. `awaitChild` additionally hard-fails when a child reports `completed` but its outcome artifact is missing (`:965-994`, `:1080-1138`).

### 3. Wave orchestrator (`internal/core/run/parallel/`) — silent segments and unclassified exits

- Happy path per wave: `runOneWave` → `runWave` (bounded goroutines) → optional recovery → `mergeWave` (serial squash-merge + agentic conflict resolution) → `emitWaveCompleted` (`orchestrator.go:397-456`). Between waves, the only bridge is `worktrees.Head(...)` (`:388-391`, verified); after the last wave, `finalize` fast-forwards the base branch, syncs artifacts best-effort, and cleans up (`:477-505`).
- **No events are emitted during finalize** (FastForward / artifact sync / worktree removal / integration discard), and there is no orchestrator-level "run completed" kind — `task.parallel.wave_completed` is the last parallel signal a consumer sees on success.
- Several error paths return without classifying the outcome or emitting a terminal parallel event: `Head` failure after a completed wave, prepare/execute join errors, non-scope `CommitTask` errors, squash transport errors, FSM transition errors (`orchestrator.go:318-345`, `:552-625`, `:628-705`). `Outcome.Status` stays empty on those paths; the daemon ignores the typed outcome and only emits `task.multi.queue_completed` when `Run` returns nil (`internal/daemon/task_multi.go:639-670`).
- Failed/skipped tasks never get per-task terminal parallel events; only merged tasks do (`orchestrator.go:703`, helpers `:1094-1177`). `task.parallel.task_started` is emitted by the daemon launcher, not the orchestrator (`internal/daemon/task_multi.go:928-951`).
- `cleanupCompleted` removes a task worktree only for outcomes `merged`/`recovered` (`orchestrator.go:1186-1188`) with a **non-force** remove; a failed `Remove` returns early and **skips the integration-branch discard** (`:815-836`).

### 4. Worktree lifecycle on disk

- Worktrees live under the Compozy home, not the workspace: `WorktreesDir = <home>/state/worktrees` (`internal/config/home.go:113`, verified), path formula `<worktreesRoot>/<workspaceHash12>/<parentShort12>-<parentHash8>/<NN>-<slug>` plus an `integration` leaf and branch `compozy/parallel-<runID>` (`internal/daemon/task_multi_worktree.go:717-757`; `internal/daemon/task_multi.go:451-474`).
- Every parent run id gets a fresh directory tree; target collision is an error, nothing is reused (`task_multi_worktree.go:800-808`). Recovery/restart reuses the existing allocation (`internal/daemon/task_multi.go:903-919`).
- Statuses that keep worktrees on disk: everything except `merged`/`recovered` (failed, skipped, canceled, rolled-back — rollback discards only the integration branch, `orchestrator.go:775-795`). Every allocation is labeled `WorktreeStatus: "preserved"` at create time (`task_multi_worktree.go:193`).
- Purge runs only via `compozy runs purge` (retention: 14 days / 200 runs — `internal/daemon/reconcile.go:22-23`), rebuilds targets from run events, and refuses dirty trees, trees with unretained commits, and paths outside the current worktrees root (`internal/daemon/worktree_purge.go:335-551`). Daemon startup reconcile marks interrupted runs crashed but never touches disk; shutdown never cleans worktrees (`reconcile.go:96-174`, `shutdown.go:210-261`).
- Child isolation is cwd + sanitized git env (`internal/core/gitenv/gitenv.go:59-74`), not a sandbox; an agent using absolute paths can still write to the original workspace.

### 5. Wizard (`compozy tasks run` interactive)

- Selection step is checkbox-style: arrows move a cursor, Space toggles, Enter advances (`internal/cli/tasks_run_wizard.go:635-670`, `toggleWorkflow :1383-1396`). Cursor, toggle, and rendering all index the same `filteredWorkflowOptions()` slice — no index/filter desync found; tests cover filter+toggle (`tasks_run_wizard_test.go:18-37`, `:297-311`).
- Interaction traps that match the B1 report: focus moved to the RUN ORDER pane (right/`l`) keeps accepting arrows but **silently ignores Space** (`:591-632`); Enter advances the step **without toggling** the highlighted row (`:663-664`).
- Execution step: "Run Parallel Tasks" (within-spec waves; triggers worktrees for a single spec via §1) is always shown; "Run Parallel Workflows" (multi-spec worktrees) appears **only when 2+ workflows are selected** (`executionFields :759-791`). The only "worktree" wording in the whole wizard is that field's description (`:1899-1907`). There is no explicit mode display ("enqueued/serial vs parallel/worktrees") and no independent worktree yes/no control.
- Mapping: wizard → `state.parallel`/`state.parallelTasks` → `resolveTaskRunMultipleMode` (`internal/cli/daemon_commands.go:1031-1048`, verified: flag changed ⇒ explicit; otherwise workspace `run_multiple_mode` decides) → `TaskRunMultipleRequest.Mode` (`internal/api/contract/types.go:48-55`). The wizard marks `--parallel` changed when applying 2+ selections; non-wizard `--multiple` without `--parallel` silently inherits workspace `run_multiple_mode=parallel` when set.

### 6. Client attach/observe (TUI, `--stream`, watchers)

- Multi-run attach loads a snapshot, then opens the parent event stream **at zero cursor** (`internal/core/run/ui/multi_remote.go:174-182`, `:414`), replaying the parent journal from seq 0 over already-applied snapshot state.
- On `task.parallel.wave_completed` the UI marks the wave glyph merged but leaves job cards and the tab status `running`; job finish is keyed to per-task `merged` and parent terminal events only (`internal/core/run/ui/integration.go:327-340`, `multi_remote.go:842-861`, `:990-1044`).
- Stream lifetime: `followRemoteRun` stops only on `run.completed|failed|cancelled|crashed`; `task.multi.queue_completed` and all `task.parallel.*` kinds do not stop it (`internal/core/run/ui/remote.go:671-680`). `waitRemoteCLIRunUI` waits only for TUI exit — no run-status polling, no stream-silence watchdog (`internal/cli/run_observe.go:320-356`). Auto-quit requires `isQueueComplete()` (parent terminal or all tabs terminal — `multi_remote.go:1837-1849`, verified).
- Textual `--stream` renders `task.multi.*` but **zero** `task.parallel.*` kinds (`internal/cli/run_observe.go:635-681`); lean JSON allowlist omits both families entirely (`docs/events.md:918-930`, verified).
- Run-TUI navigation: arrows move focus over the flat `jobs[]` slice while the sidebar renders jobs regrouped under wave headers — visual order and navigation order can diverge, so cards appear "skipped" (`internal/core/run/ui/update.go:508-534`, `integration.go:371-414`). There is no multi-select in the run TUI; selection-for-execution exists only in the wizard.
- Quit semantics differ: attach-from-start cancels the run on quit; `runs attach` does not (`internal/cli/run_observe.go:84-96`, `:358-386`).

### 7. Event/API contract

- Kinds: 8 `task.multi.*` + 10 `task.parallel.*` public kinds (`pkg/compozy/events/event.go:74-93`, payloads in `pkg/compozy/events/kinds/task.go:45-118`), documented in `docs/events.md:408-619` with drift-guard tests.
- Journal is append-before-publish, but **fsync-before-publish applies only to `run.*` terminals** (`internal/core/run/journal/journal.go:756-765`, verified: `isTerminalEvent` lists only `run.crashed|completed|failed|cancelled`); queue/wave kinds can be dropped under submit backpressure.
- `TaskRunMultipleSnapshot{Run, Items}` has no `NextCursor`/`Incomplete` gap metadata, unlike `RunSnapshotResponse` (`internal/api/contract/types.go:57-71` vs `:612-626`), and its items are rebuilt from `task.multi.*` only — wave/task state is not reconstructable from the snapshot endpoint.
- Terminal vocabulary is split: multi TUI settles on `queue_completed|queue_canceled` (`multi_remote.go:1147-1155`) while `WatchRemote`/CLI watchers settle only on `run.*` (`pkg/compozy/runs/remote_watch.go:334-343`, `internal/cli/reviews_exec_daemon.go:1257-1266`); an older helper `consumeDaemonRunStream` opens once at empty cursor and never resumes on disconnect (`reviews_exec_daemon.go:902-938`).

## Evidence

- User report: verbatim Portuguese quote in Problem section (secondhand, from support/user channels; no session transcripts available).
- Runtime logs / stack traces: **Unknown — no runtime reproduction was executed for this brief; all findings are from static analysis, with the quoted excerpts read directly from source and 8 additional citations sample-verified (one per analysis slice).**
- Verified code excerpts: `task_multi.go:121-124` gate and `:1828-1849` poll loop (quoted above); `daemon_commands.go:1031-1048` mode resolution; `orchestrator.go:385-394` post-wave `Head`; `home.go:107-120` `WorktreesDir`; `task_multi_worktree.go:717-726` path planner; `multi_remote.go:1837-1849` `isQueueComplete`; `journal.go:756-765` `isTerminalEvent`; `docs/events.md:918-930` lean allowlist.
- Derived reproduction sketches (static, not yet executed — the receiver must reproduce before fixing):
  - **B3:** set `[tasks.run.parallel] enabled = true` in workspace TOML (or pass `--parallel-tasks`), run `compozy tasks run <one-slug>`; expect one detached worktree per task plus an `integration` worktree under `~/.compozy/state/worktrees/<wsHash>/<runSeg>/`, freshly created again on every re-run.
  - **B2 (daemon):** during a parallel run, kill a child agent process so it never writes a terminal run status; parent poll loop never returns; TUI shows a finished wave then nothing.
  - **B2 (client):** run a plan with ≥2 waves and watch after `wave_completed`: jobs stay `running` during merge/finalize with no further events; in `--stream` mode a `--parallel-tasks` run prints nothing between agent output and final `run.*`.
  - **B1 (wizard):** select item, press right (focus RUN ORDER), move arrows, press Space — nothing toggles; or highlight a row and press Enter — step advances, row not selected.
  - **B5 (leak):** make any task fail (or cancel mid-wave); its worktree remains on disk indefinitely unless `compozy runs purge` later succeeds on a clean tree.
- Existing e2e intent: `internal/cli/tasks_run_parallel_e2e_test.go:104-127`, `:173-188` (worktree-backed multi-run and single-spec parallel preflight).
- Full evidence base: six seven-section analyses + synthesis in `.compozy/tasks/worktree-parallel-bugs/analysis/` (paths in References).

## Requirements

What must be true when the work is done. Testably stated; grouped by cluster.

### B2 — every run reaches a visible terminal state (highest priority)

1. For every ending — success, task failure, merge-conflict exhaustion, child process death, cancel, daemon restart mid-run — the parent run must reach a terminal status **and** a terminal event observable by every supported consumer (multi TUI attach, single-run attach, `--stream` text, lean JSON, `WatchRemote`, snapshot polling) within a bounded, configurable time after work stops.
2. A parent run must never wait indefinitely on a child that can no longer make progress; abnormal child death must surface as a terminal child status and be observed by the waiting wave.
3. The orchestrator must not have any exit path that returns without a classified outcome (`completed|canceled|failed|rolled_back`) and its corresponding terminal event; wave completion must never be the final observable signal of a run — consumers must be able to distinguish "progressing", "finishing/finalizing", and "terminal".
4. Post-wave and finalize phases (base advancement, fast-forward, artifact sync, cleanup) must be observable while they run; a consumer watching any supported surface must see activity or a documented waiting state, never unexplained silence.
5. The observing CLI must terminate (or explicitly display why it is still waiting) once the run is settled; a silent event stream with a non-terminal run must be detectable by the client rather than hanging forever.
6. `--stream` text mode and lean JSON must convey the parallel-tasks lifecycle (or their exclusions must be documented and a documented alternative given); a parallel run must never look "dead" in a supported observation mode while it is progressing.
7. Queue/wave settlement signals must be durable enough that a consumer cannot permanently miss the end of a queue/wave that the run itself recorded (today only `run.*` kinds are fsync-before-publish).

### B3 — worktree use is a choice, and its footprint is bounded

8. Running a single spec must not silently switch into worktree-backed execution: the user must have made a per-run choice (flag/wizard) or have an explicit, visible default; whichever policy is chosen, the resolved execution mode must be displayed to the user before/at run start.
9. Ambient workspace TOML must not be able to force worktrees on a run against the user's per-run intent; per-run intent must always win, and the interaction between TOML default and per-run choice must be documented.
10. Re-running a spec must not accumulate unbounded worktree directories: terminal runs must have a defined cleanup story (immediate removal when safe, or preserved-with-reason and collectable), including failed/skipped/canceled/rolled-back outcomes, crashed runs after daemon restart, and the case where removing one task worktree fails while others (and the integration worktree) remain removable.
11. Worktree status labels shown to users/APIs must reflect reality (a tree slated for automatic cleanup must not be labeled the same as one intentionally preserved for review).
12. Relocating `COMPOZY_HOME` must not silently strand git-registered worktrees; the system must either handle or clearly surface stale registrations from a previous home.

### B4 — explicit multi-spec worktree choice

13. When 2+ specs are selected in the wizard, the user must be presented an explicit, discoverable choice between serial (enqueued) and parallel-in-worktrees execution, with wording that says worktrees are involved; the review/summary step must show the resolved mode.
14. Non-interactive `--multiple` runs must state the resolved mode (and its source: flag vs workspace config) so `run_multiple_mode = "parallel"` in TOML can never surprise the user.
15. The single-spec "Run Parallel Tasks" option must communicate that it uses git worktrees and what disk/branch side effects it has, and must honor requirement 8.

### B1 — selection does what the user believes

16. In the wizard workflow list, every interaction that looks actionable must act or visibly explain itself: Space while RUN ORDER pane is focused must not be a silent no-op, and Enter must not advance while leaving the highlighted row's state ambiguous. The final selected set shown at the review step must equal what the user confirmed.
17. In the run TUI, arrow navigation over wave-grouped items must follow the visual order presented on screen (no apparent skipping).
18. Regression coverage must pin the wizard interaction set: toggle under active filter, toggle-all under filter, reorder, focus switch, Enter-with-unselected-highlight, and selection survival across filter changes.

### B5 — one coherent contract

19. One documented terminal ladder for multi/parallel runs (what `queue_*` means vs `run.*`), shared by the TUI, CLI watchers, and `pkg/compozy/runs`; no consumer may implement its own divergent terminal rule (the legacy non-resuming stream helper must comply or be retired).
20. An attaching client must neither miss nor double-apply parent events around the snapshot→stream boundary; the multi snapshot must be sufficient to detect parent-journal gaps (parity with the single-run snapshot's integrity signals) and to restore the parallel-tasks view (or its insufficiency must be documented with the sanctioned recovery path).
21. The two parallel products (within-spec waves vs multi-spec queue) must present distinguishable identity to users and machines (mode labels, snapshot fields, events), with documented and intentional differences in failure policy and recovery policy — or those policies must be unified; today's silent asymmetries (fail-fast vs fail-late, recovery force-disabled vs enabled) must not remain implicit.
22. Quit semantics (does quitting the TUI cancel the run?) must be consistent or clearly communicated per entry point.
23. `docs/events.md` (including the lean-mode allowlist and the 74-kind inventory) must match the code after the changes; existing docs drift-guard tests must stay green and cover any new/changed kinds or fields.

## Constraints & Non-goals

- **Verification gate:** `make verify` (fmt + lint + test + build) must pass 100%; golangci-lint zero issues; race detector on tests (repo policy, CLAUDE.md).
- **Concurrency discipline (repo policy):** every goroutine has explicit ownership and ctx-based shutdown; no fire-and-forget; no `time.Sleep` as synchronization in orchestration paths.
- **Public API compatibility:** `pkg/compozy/events` and `pkg/compozy/runs` are public. Existing kind names and payload JSON shapes must remain backward compatible (`payload_compat_test.go`, `docs_test.go` guard this); additive changes are acceptable, breaking changes require explicit versioning per the envelope's `schema_version` story.
- **HTTP/API compatibility:** OpenAPI contract tests (`internal/api/httpapi/openapi_contract_test.go`, `internal/api/client/client_contract_test.go`) pin the current surface; extensions must keep existing clients working.
- **Data safety:** purge/cleanup guardrails exist deliberately (dirty-tree refusal, unretained-commit refusal, ownership checks under the worktrees root). Any strengthened cleanup must never delete user work that is not provably Compozy-owned and reproducible; destructive git operations on the user's primary workspace remain out of bounds.
- **Architecture:** single-binary, local-first; no sidecars or external control planes (repo runtime discipline). Keep execution deterministic and observable.
- **Existing behavior to preserve:** enqueued multi-spec mode (serial, no worktrees) keeps working as-is; `--parallel-tasks` + `--multiple` rejection stays unless a spec deliberately changes it; recovery/restart reusing an existing allocation (no duplicate worktree per retry) is correct today and must not regress.
- **Non-goals:** merging the two parallel products into one engine; redesigning the Bubble Tea UI beyond the behaviors named in Requirements; sandboxing agents against absolute-path escapes (documented as a known limitation — see analysis 04); changing the worktrees-under-home layout.

## Verification

- Reproduce first: execute the derived reproduction sketches in Evidence against a real daemon and at least one real multi-wave spec before changing code; capture the actual behavior (run status in global DB during the stall — this discriminates daemon-hang vs client-freeze and must be recorded in the fix's findings).
- Layer coverage expected (reuse existing canonical suites; do not create parallel one-off suites):
  - `internal/core/run/parallel/orchestrator_test.go`: every orchestrator exit path yields a classified outcome + terminal emit (Head failure after a wave, finalize failure, transition failure, cancel mid-wave, resolver exhaustion); post-wave/finalize observability; failed/skipped task terminal signals.
  - `internal/daemon/task_multi_test.go`: single-spec routing honors per-run intent vs TOML; child death without terminal status is bounded and surfaces; completed-child-with-missing-outcome policy; mode resolution and its visibility.
  - `internal/cli/tasks_run_wizard_test.go`: the B1 interaction set (requirement 18) and the B4 mode control incl. review-step display.
  - `internal/core/run/ui/multi_remote_test.go` / `integration_test.go`: wave_completed handling, terminal ladder, snapshot→stream boundary idempotency, visual-order navigation.
  - Contract: `pkg/compozy/events` docs/compat tests, OpenAPI + client contract tests, `docs/events.md` drift guards updated together.
  - E2E: extend `internal/cli/tasks_run_parallel_e2e_test.go` to cover: single spec with and without the per-run worktree choice; multi-spec enqueued vs parallel; a full run whose observer exits cleanly at terminal; worktree cleanup state on disk after terminal runs (success and failure).
- Final proof: `make verify` output at 100%, plus a manual end-to-end session (start daemon, run a ≥2-wave spec in each mode, observe TUI + `--stream` to natural exit, inspect `~/.compozy/state/worktrees` before/after) demonstrating each of the five clusters no longer reproduces.

## References

- Research round (read these first — each is a seven-section analysis with file:line evidence):
  - `.compozy/tasks/worktree-parallel-bugs/analysis/summary.md` — cross-slice synthesis, convergences/divergences
  - `.compozy/tasks/worktree-parallel-bugs/analysis/01_analysis_wizard-selection-worktree.md`
  - `.compozy/tasks/worktree-parallel-bugs/analysis/02_analysis_wave-orchestrator-stall.md`
  - `.compozy/tasks/worktree-parallel-bugs/analysis/03_analysis_daemon-task-multi-modes.md`
  - `.compozy/tasks/worktree-parallel-bugs/analysis/04_analysis_worktree-lifecycle-cleanup.md`
  - `.compozy/tasks/worktree-parallel-bugs/analysis/05_analysis_run-tui-observe.md`
  - `.compozy/tasks/worktree-parallel-bugs/analysis/06_analysis_events-contract-consistency.md`
- Key source files: `internal/daemon/task_multi.go` (`:74-179`, `:490-577`, `:616-670`, `:928-994`, `:1187-1374`, `:1828-1849`), `internal/daemon/run_manager.go` (`:421-459`, `:2595-2681`), `internal/daemon/task_multi_worktree.go` (`:161-195`, `:471-507`, `:631-653`, `:717-808`), `internal/daemon/worktree_purge.go`, `internal/daemon/reconcile.go`, `internal/daemon/shutdown.go`, `internal/core/run/parallel/orchestrator.go` (`:318-505`, `:552-705`, `:775-837`, `:1094-1188`), `internal/core/run/parallel/events.go`, `internal/cli/tasks_run_wizard.go` (`:501-670`, `:718-809`, `:1369-1441`, `:2310-2480`), `internal/cli/daemon_commands.go` (`:589-886`, `:1031-1048`, `:1315-1358`), `internal/cli/run_observe.go`, `internal/core/run/ui/{model,update,integration,multi_remote,remote}.go`, `internal/core/run/journal/journal.go`, `internal/api/contract/types.go`, `pkg/compozy/events/`, `pkg/compozy/runs/remote_watch.go`, `internal/config/home.go`, `docs/events.md`.
- History: PR #200 (feat: worktree-backed parallel multi-run, 6fa1d4b), PR #217 (fix: parallel execution, cc626ba), PR #223 (fix: worktree management, 1219df3), PR #216 (COMPOZY_HOME override, 73ced7f).
