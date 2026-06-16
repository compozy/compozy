# Task 09 Memory

## V2: Render Parallel Worktree Handoff in TUI and CLI Output

Surfaced worktree metadata wherever a user observes a parent multi-run: the
tabbed TUI, the non-UI stream, and a new final handoff summary. No new dashboard;
the existing tabbed attach surface and child transcript model are preserved.

### What changed (source)
- `internal/core/run/ui/multi_remote.go`
  - `multiRunTab` gained `worktreePath`/`baseBranch`/`baseCommit`/`worktreeStatus`.
  - `newMultiRunTab(*apicore.TaskRunMultipleItem)` copies the snapshot worktree
    fields; `applyTaskMultiItem` merges later parent-event worktree fields via new
    `applyTaskMultiWorktreeMetadata` (non-empty overwrites only — empty preserves
    prior value, mirroring the daemon snapshot builder, so a metadata-free child
    completed/failed event never erases the path).
  - New persistent worktree status line under the tab strip
    (`renderActiveTabWorktreeLine` + pure `formatMultiRunWorktreeSummary` /
    `multiRunWorktreeLabel`) showing the SELECTED tab's `worktree <status> <path>
    branch <b> run <id>`. Missing metadata renders an em dash (`worktree —`).
  - `multiRunTabsHeight` 2 -> 3 (tab line + worktree line + separator); childHeight
    accounting flows from that constant, so no other layout edits were needed.
- `internal/cli/run_observe.go`
  - `renderObservedTaskMultiItem` refactored from a 4-way switch to segment-join;
    now appends `worktree=<path>` for child events (existing outputs unchanged when
    path is empty). Order: `label status | run=<id> | worktree=<path> | <error>`.
  - New final handoff: `writeTaskRunMultipleHandoff` (fetches
    `GetTaskRunMultipleSnapshot`) -> `renderTaskRunMultipleHandoff` -> pure
    `formatTaskRunMultipleHandoff`/`formatTaskRunMultipleHandoffItem`. Children in
    requested (snapshot) order; missing run id / worktree render as `-`
    (`handoffValueOrDash`). Range snapshot items BY INDEX (gocritic rangeValCopy —
    item is 128 bytes).
- `internal/cli/daemon_commands.go`
  - `handleStartedTaskRunMultiple` now delegates the stream path (and the
    UI-settled fallback) to new `streamTaskRunMultipleToTerminal`, which always
    writes the handoff AFTER `watchCLIRunUntilTerminalSuccess`, then returns the
    watch error so aggregate failed/canceled/crashed still exits non-zero (exit 1).
    Handoff is best-effort: the primary watch error wins.

### Key decisions
- One persistent header line is the "status area" for the selected child (covers
  queued/running/terminal). Not duplicated into the queued panel to avoid
  redundancy — the header always reflects the active tab.
- Em dash for empty TUI metadata; `-` for empty CLI handoff fields (both satisfy
  the "empty/unknown for backward compatibility" requirement).
- `parallel_limit` is intentionally NOT rendered per-child (it is not on snapshot
  items; it only rides the parent `started` event — observability only).

### Tests (all `-race` clean)
- UI: `TestMultiRunInitialSnapshotRendersWorktreeForSelectedChild`,
  `TestMultiRunChildStartedEventAppliesWorktreeMetadataToTab` (incl.
  preserve-on-metadata-free-event), `TestMultiRunSnapshotWithoutWorktreeMetadataRendersDash`,
  `TestFormatMultiRunWorktreeSummary` (table: nil/none/path/status/full).
- CLI unit: `TestRenderObservedTaskMultiItemIncludesWorktreePath`,
  `TestFormatTaskRunMultipleHandoff` (order + dash + nil).
- CLI integration (stub stream + snapshot, sleep-free):
  `TestTasksRunMultipleStreamPrintsWorktreeHandoff` (success: child-started lines
  carry `worktree=`, handoff prints both paths) and
  `...StreamReportsAggregateFailureWithPaths` (mixed result: exit 1, `run failed |
  ...`, handoff `beta failed | ... worktree=/wt/02-beta | boom`). Helpers
  `taskMultiStreamEvent`/`taskMultiStreamTerminalEvent`/`runTasksRunMultipleStreamScenario`
  reuse `staticClientRunStream` + `stubDaemonCommandClient`.

### Handoff
task_10 (docs/e2e) should document the final handoff output shape (`task multi-run
handoff:` + per-child `slug status | run=<id> | worktree=<path>`), the TUI worktree
status line, and that stream child events now carry `worktree=`. See [[task_08]]
for the concurrent scheduler this renders.
