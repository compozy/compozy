## TC-UI-001: Manual TUI Operator Confirmation

**Priority:** P1 (High)
**Type:** UI
**Status:** Not Run
**Estimated Time:** 20 minutes
**Created:** 2026-04-18
**Last Updated:** 2026-04-18
**Automation Target:** Manual-only
**Automation Status:** N/A
**Automation Command/Spec:** Real-terminal validation of `compozy tasks run <slug> --ui` plus `compozy runs attach <run-id>` against a daemon-managed fixture workflow.
**Automation Notes:** Supporting automated state/render guards already exist in `internal/core/run/ui/remote_test.go`, `internal/core/run/executor/execution_ui_test.go`, and `internal/core/run/ui/view_test.go`, but there is no stable full-screen terminal acceptance harness for operator feel/readability on this branch.

### Objective

Confirm that the daemonized TUI still feels operator-ready in a real terminal session: readable layout, stable navigation, attach/reattach continuity, and sensible completion behavior.

### Preconditions

- [ ] A real TTY is available.
- [ ] A daemon-managed workflow fixture can emit enough events to exercise sidebar/timeline updates.
- [ ] The executor already ran `TC-FUNC-002` and `TC-FUNC-006` successfully before this manual confirmation.

### Test Steps

1. Start a daemon-managed workflow in a real terminal with `compozy tasks run <slug> --ui`.
   **Expected:** The TUI attaches by default, renders initial snapshot state, and remains readable without visual corruption.

2. Resize the terminal to at least one narrow and one standard width while the run is active.
   **Expected:** Sidebar, summary, and timeline remain usable; focus/navigation state does not become inconsistent.

3. Quit the active UI client, then reattach with `compozy runs attach <run-id>`.
   **Expected:** The TUI restores from daemon snapshot state and resumes the same run without duplicated timeline events.

4. Let the run complete and observe the terminal state.
   **Expected:** Final state is understandable, completion behavior is stable, and attach to a completed run exits cleanly after rendering the final snapshot.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Narrow terminal | Reduced width/height | Layout degrades gracefully and stays readable |
| Completed-run attach | Attach after run is terminal | Final snapshot renders without hanging |
| Remote EOF after completion | Stream closes after terminal state | UI exits cleanly and predictably |

### Related Test Cases

- `TC-FUNC-002`
- `TC-FUNC-006`

### Traceability

- ADR-004: preserve TUI-first UX while introducing daemon attach semantics.
- Task reference: `task_12`.
- Supporting automated guards: `internal/core/run/ui/remote_test.go`, `internal/core/run/executor/execution_ui_test.go`, `internal/core/run/ui/view_test.go`.

### Notes

- Capture screenshots only if they materially help explain a regression. Do not invent a browser lane from this case.
