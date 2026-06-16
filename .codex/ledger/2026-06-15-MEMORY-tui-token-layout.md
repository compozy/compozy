# TUI Token Layout Ledger

## Goal (incl. success criteria):

Implement the accepted TUI token/layout plan: compact token displays use provider `total_tokens`, the global SYS.PIPELINE header is removed, progress/tokens move into the jobs sidebar, focused regressions prove the real screenshot payload behavior, `make verify` passes, and `cy-impl-peer-review` returns SHIP.

## Constraints/Assumptions:

- Prefix shell commands with `rtk`.
- Do not run destructive git commands or revert unrelated tracked/untracked files.
- Preserve existing uncommitted changes from prior task-number/sidebar work.
- Compact token metric is provider total: `Usage.Total()`, including cache when the provider reports cache tokens.
- Public event/storage schemas remain unchanged.

## Key decisions:

- Accepted plan persisted at `.codex/plans/20260614-230509-tui-token-layout.md`.
- Root cause: the run header showed `total_tokens` while the job card showed only input/output split, hiding cache reads/writes.
- Fix visual contract and cache invalidation rather than recalculating provider totals.

## State:

Implementation patched; focused and changed-package tests passed; full verification and peer review pending.

## Done:

- Read existing ledgers and relevant prior task-number memory.
- Read RTK guidance.
- Loaded required skills: systematic-debugging, no-workarounds, tui-design, tui-glamorous, golang-pro, testing-anti-patterns.
- Investigated real advos run artifact `tasks-design-gaps-6c79aa-20260615-005417-723920000-88b6dcd6aacef4de`.
- Confirmed payload: input 22139, output 190016, cache_reads 31873174, cache_writes 405208, total 32490537.
- User chose Total provider metric.
- Persisted accepted plan.
- Added focused regressions for provider-total labels, removed global pipeline chrome, sidebar status/progress/tokens, ACP usage conversion comments, and sidebar usage-update invalidation.
- Patched TUI header/sidebar/timeline rendering so compact token labels use `Usage.Total()` and progress/tokens live under `JOB`.
- Focused validation passed: `rtk go test ./internal/core/run/ui ./internal/core/agent -run 'Test(JobsViewUsesACPChromeWithoutInspectorPane|ResizeGateAppearsBelowMinimumTerminalSize|TimelineRuntimeMetaFallbacks|FormatUsageTotalLabel|SidebarStatusAndShutdownLabels|SidebarTitleShowsProgressAndAggregateTokens|HandleUsageUpdateRefreshesSidebarProviderTotal|ConvertACPUsage)' -count=1`.
- Changed-package validation passed: `rtk go test ./internal/core/run/ui ./internal/core/agent ./internal/core/plan ./internal/core/tasks ./internal/daemon -count=1`.

## Now:

Run full `make verify`, then `cy-impl-peer-review` until SHIP.

## Next:

Address any full verification or peer-review findings, then final report.

## Open questions (UNCONFIRMED if needed):

None.

## Working set (files/ids/commands):

- `.codex/plans/20260614-230509-tui-token-layout.md`
- `.codex/ledger/2026-06-15-MEMORY-tui-token-layout.md`
- Expected code files: `internal/core/run/ui/view.go`, `sidebar.go`, `timeline.go`, `types.go`, `update.go`, `view_test.go`, `update_test.go`, `internal/core/agent/acp_convert.go`, `internal/core/agent/acp_convert_test.go`.
