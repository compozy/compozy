Goal (incl. success criteria):

- Restore daemon-backed `tasks run` cockpit parity for two regressions:
  - interactive per-task runtime selections must reach daemon execution and appear in the cockpit;
  - moving the left sidebar selection must immediately swap the right detail pane back to the selected task.
- Success means focused regressions pass and fresh `make verify` passes.

Constraints/Assumptions:

- Follow `AGENTS.md` and `CLAUDE.md`.
- Required skills this session: `systematic-debugging`, `no-workarounds`, `bubbletea`, `golang-pro`, `testing-anti-patterns`; `cy-final-verify` before claiming completion.
- Do not use destructive git commands or touch unrelated worktree changes.
- Fix root causes only; do not add persistence under workflow task directories for execution-local selections.

Key decisions:

- Treat the missing runtime selection as a daemon override handoff bug, not a workflow artifact persistence bug.
- Treat the stale right pane as a shared Bubble Tea viewport mount/cache bug, not a task-run-specific rendering bug.
- Keep the current daemon transport contract (`task_runtime_rules` in runtime overrides) unchanged.

State:

- Completed and verified.

Done:

- Read the required skill instructions and relevant prior ledgers.
- Isolated root cause 1:
  - `collectStartTaskRuntimeForm()` updates `state.executionTaskRuntimeRules`;
  - `buildTaskRunRuntimeOverrides()` only serializes `TaskRuntimeRules` when Cobra thinks `--task-runtime` changed;
  - in pure TUI flows that flag never becomes explicit, so the daemon never receives the overrides.
- Isolated root cause 2:
  - `moveSelectedJob()` changes `selectedJob`;
  - `renderTimelinePanel()` only pushes transcript viewport content on cache miss;
  - when the newly selected job already has a valid cached render, the viewport keeps showing the previous job's content.
- Persisted the accepted implementation plan under `.codex/plans/2026-04-20-task-run-tui-parity.md`.
- Marked the second-round task-runtime form as an explicit source by calling `markInputFlagChanged(cmd, "task-runtime")` after it applies state.
- Fixed daemon task-run override serialization in `internal/cli/daemon_commands.go`:
  - task runtime overrides now serialize when either `--task-runtime` was set or the interactive form replaced configured rules;
  - explicit clears now marshal as `task_runtime_rules: []` instead of disappearing.
- Added CLI regressions in `internal/cli/form_daemon_overrides_test.go` for:
  - interactive task runtime rules propagating without direct flag mutation;
  - interactive clear sending an explicit empty rule list.
- Added shared timeline mount state in the UI model so cached timeline renders are remounted when the selected job changes.
- Cleared mounted timeline state when the job list empties.
- Added `TestRenderTimelinePanelRemountsCachedTranscriptWhenSelectedJobChanges` in `internal/core/run/ui/view_test.go`.
- Ran focused validation successfully:
  - `go test ./internal/cli ./internal/core/run/ui -run 'Test(TasksRunInteractiveFormPropagatesTaskRuntimeRulesWithoutExplicitFlagMutation|TasksRunInteractiveFormClearsConfiguredTaskRuntimeRulesExplicitly|RenderTimelinePanelRemountsCachedTranscriptWhenSelectedJobChanges|RenderTimelinePanelSkipsViewportSetContentOnCacheHit)$' -count=1`
  - result: pass
- Ran full verification successfully:
  - `make verify`
  - key output:
    - `0 issues.`
    - `DONE 2434 tests, 1 skipped in 42.236s`
    - `All verification checks passed`

Now:

- Task complete; ready for final handoff.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None at the moment.

Working set (files/ids/commands):

- `.codex/plans/2026-04-20-task-run-tui-parity.md`
- `.codex/ledger/2026-04-20-MEMORY-task-run-tui.md`
- `internal/cli/{task_runtime_form.go,daemon_commands.go,form_daemon_overrides_test.go,daemon_commands_test.go}`
- `internal/core/run/ui/{timeline.go,update.go,view_test.go,update_test.go}`
- Commands:
  - `git status --short`
  - `sed -n ... internal/cli/task_runtime_form.go`
  - `sed -n ... internal/core/run/ui/{update.go,timeline.go}`
