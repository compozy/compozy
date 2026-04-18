## TC-FUNC-004: Reviews Flow Through Daemon Lifecycle

**Priority:** P1 (High)
**Type:** Functional
**Status:** Not Run
**Estimated Time:** 15 minutes
**Created:** 2026-04-18
**Last Updated:** 2026-04-18
**Automation Target:** E2E
**Automation Status:** Existing
**Automation Command/Spec:**
- `go test ./internal/cli -run 'Test(ReviewsCommandFetchListShowUseDaemonRequests|ReviewsFixCommandResolvesLatestRoundAndBuildsDaemonRequest|ReviewsFixCommandAutoAttachStreamsWhenNonInteractive)' -count=1`
- Supporting seam: `go test ./internal/daemon -run 'Test(TransportReviewServiceFetchQueriesAndStartRunUseDaemonState|RunManagerReviewRunWatcherSyncsOwnedWorkflowArtifacts)' -count=1`
**Automation Notes:** This lane proves the public review surfaces now route through daemon sync and run lifecycle while authored Markdown review artifacts remain authoritative in the workspace.

### Objective

Verify that `reviews fetch|list|show|fix` now behave as daemon-backed flows, with pre-run sync, workspace-authored review artifacts, and stable operator-facing behavior.

### Preconditions

- [ ] Review fixtures or extension-backed review-provider stubs are available to the existing test suite.
- [ ] The workspace contains at least one review round and issue set for the targeted scenario.

### Test Steps

1. Run the focused reviews CLI suite listed above.
   **Expected:** The package exits `0` and the public review flows pass through daemon requests.

2. Confirm the suite covers latest-round resolution and non-interactive stream behavior for `reviews fix`.
   **Expected:** Review fix resolves the correct round, uses the daemon run lifecycle, and stays stable outside a TTY.

3. Run the supporting daemon review-service suite when deeper lifecycle evidence is needed.
   **Expected:** Fetch/show/fix use daemon state, and review-run watchers keep workspace-authored review artifacts aligned.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Latest round resolution | Multiple review rounds | `reviews fix` targets the expected most recent round |
| Non-interactive review fix | No TTY available | Stream mode remains usable without TUI |
| Manual review artifact edits | Workspace review files change during run | Daemon-backed review state stays aligned without relocating authored artifacts |

### Related Test Cases

- `TC-FUNC-005`
- `TC-INT-002`

### Traceability

- TechSpec Integration Tests: `reviews fix` auto-syncs before execution and keeps live review issue state aligned with manual file edits.
- ADR-002: human-authored review artifacts remain in the workspace.
- ADR-003: review flows use the daemon transport contract.
- Task reference: `task_15`.

### Notes

- Treat authored Markdown review files as the human-facing source, with daemon storage mirroring their operational state.
