## TC-FUNC-002: Task Runs, Attach Mode, and Watcher Sync

**Priority:** P0 (Critical)
**Type:** Functional
**Status:** Not Run
**Estimated Time:** 18 minutes
**Created:** 2026-04-18
**Last Updated:** 2026-04-18
**Automation Target:** E2E
**Automation Status:** Existing
**Automation Command/Spec:**
- `go test ./internal/cli -run 'Test(TasksRunCommandDispatchesResolvedWorkspaceAndConfiguredAttachMode|TasksRunCommandAutoModeResolvesToStreamInNonInteractiveExecution|TasksRunCommandInteractiveUIModeAttachesThroughRemoteClient|TasksRunCommandExplicitUIFailsWithoutTTY|TasksRunCommandBootstrapFailureReturnsStableExitCode)' -count=1`
- Supporting seam: `go test ./internal/daemon -run 'Test(RunManagerStartTaskRunAllocatesRunDBAndRejectsDuplicateRunID|RunManagerTaskRunWatcherSyncsTaskEditsAndStopsOnCancel|RunManagerTaskRunWatchersStayIsolatedAcrossWorkflows)' -count=1`
**Automation Notes:** The public CLI suite proves operator-visible behavior. The supporting daemon-manager suite proves watcher sync and duplicate-run protection when root-cause evidence is needed.

### Objective

Verify the canonical daemon-native workflow path: `compozy tasks run <slug>` auto-syncs before execution, resolves attach mode correctly, binds to the daemon run lifecycle, and keeps workflow edits synchronized during active runs.

### Preconditions

- [ ] The workspace fixture contains a runnable daemon workflow.
- [ ] The environment can run both non-interactive and interactive-mode CLI tests.
- [ ] The temp home/runtime directory is isolated for the execution slice.

### Test Steps

1. Run the focused `tasks run` CLI suite listed above.
   **Expected:** The package exits `0` and all listed subtests pass.

2. Confirm the suite covers non-interactive auto mode, interactive UI mode, explicit UI-without-TTY rejection, and stable bootstrap failures.
   **Expected:** Attach-mode resolution follows the daemon contract and returns stable exit behavior.

3. Run the supporting daemon-manager suite if deeper lifecycle validation is needed.
   **Expected:** Duplicate `run_id` values are rejected, watcher sync propagates task edits during active runs, and watcher state remains isolated across workflows.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Non-interactive run | No TTY available | Auto mode resolves to stream output rather than TUI |
| Explicit UI without TTY | `--ui` in non-interactive shell | Stable validation failure |
| Duplicate run ID | Second start of same `run_id` | Conflict, not parallel execution |
| Workflow edits during run | Authored task files change while run is active | Scoped watcher sync updates daemon state without cross-workflow bleed |

### Related Test Cases

- `TC-FUNC-003`
- `TC-FUNC-006`
- `TC-UI-001`

### Traceability

- TechSpec Integration Tests: `tasks run` auto-syncs before execution and starts a scoped watcher.
- TechSpec Unit Tests: same `run_id` rejection and client-side attach mode resolution.
- ADR-004: preserve TUI-first UX while keeping explicit attach modes.
- Task references: `task_12`, `task_14`, `task_17`.

### Notes

- This case is the canonical daemon-native workflow proof. If it fails, defer review/exec and manual TUI judgment until the root cause is fixed.
