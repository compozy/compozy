## TC-FUNC-002: External Workspace Operator Flow and Run Inspection

**Priority:** P1 (High)
**Type:** Functional
**Status:** Not Run
**Estimated Time:** 18 minutes
**Created:** 2026-04-21
**Last Updated:** 2026-04-21
**Automation Target:** E2E
**Automation Status:** Existing
**Automation Command/Spec:**
- `go test ./internal/cli -run 'Test(TaskAndReviewCommandsExecuteDryRunAgainstTempNodeWorkspace|ReviewsFixCommandExecuteDryRunRawJSONStreamsCanonicalEvents|RunsAttachCommandUsesRemoteUIAttach|RunsAttachCommandFallsBackToWatchWhenRunIsAlreadySettled|RunsWatchCommandStreamsWithoutLaunchingUI)' -count=1`
**Automation Notes:** This is the required realistic operator-flow proof outside the repository’s own Go fixtures. It uses the existing temp Node.js workspace fixture plus daemon-backed run-inspection paths.

### Objective

Verify that daemon-backed task/review flows and run-inspection commands still work against a minimal external workspace fixture, not only against repository-native test data.

### Preconditions

- [ ] The existing temp Node.js workspace fixture remains available in `internal/cli/root_command_execution_test.go`.
- [ ] The execution environment can run the CLI integration suite with an isolated temp home.
- [ ] Run attach/watch surfaces remain enabled on the execution branch.

### Test Steps

1. Run the focused external-workspace CLI suite listed above.
   **Expected:** The package exits `0` and the temp Node.js workspace flow passes.

2. Confirm the suite validates daemon-backed task or review execution rather than a local-only dry-run shim.
   **Expected:** The command path exercises daemon lifecycle behavior and canonical event streaming.

3. Confirm the suite validates run inspection after execution.
   **Expected:** `runs attach` or `runs watch` surfaces remain usable for a live or settled run.

4. If a failure is found, keep the fixture realistic and minimal while reproducing the issue.
   **Expected:** Root-cause evidence points to daemon/operator behavior, not to a contrived fixture.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Non-Go workspace | Minimal Node.js API fixture | Commands still resolve workspace and task/review artifacts correctly |
| Settled run attach | Attach after run has already finished | Falls back to stable watch/read behavior |
| Raw JSON event stream | Review/task execution with raw stream expectations | Canonical event payloads stay intact |

### Related Test Cases

- `TC-INT-002`
- `TC-INT-004`

### Traceability

- TechSpec `Testing Approach`
- ADR-003: validation-first daemon hardening
- ADR-004: observability as a first-class contract
- Task reference: `task_09.md` requirement 4

### Notes

- This case is mandatory because the execution task must validate at least one daemon-backed operator flow against a temporary external workspace.
