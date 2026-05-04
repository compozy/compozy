## TC-FUNC-007: Runs Inventory, Run Detail, and Live-Watch Baseline

**Priority:** P0 (Critical)
**Type:** Functional
**Status:** Not Run
**Estimated Time:** 12 minutes
**Created:** 2026-04-21
**Last Updated:** 2026-04-21
**Automation Target:** E2E
**Automation Status:** Existing
**Automation Command/Spec:**
- `bun run frontend:e2e`
- Supporting integration: `bunx vitest run --config web/vitest.config.ts web/src/routes/-runs.integration.test.tsx`
**Automation Notes:** Playwright smoke validates that the runs console is reachable from daemon-seeded data and that the run detail page exposes stream status. The route integration suite covers filters and richer snapshot assertions.

### Objective

Verify that the runs console presents the seeded run list and a usable run detail view, and that the browser surfaces live-watch status against the daemon REST/SSE contract.

### Preconditions

- [ ] Seeded run IDs are available from `web/e2e/global.setup.ts`.
- [ ] Run snapshots are reachable from the daemon HTTP listener.
- [ ] The stream factory override used in integration tests remains available for deterministic route checks.

### Test Steps

1. Open `/runs` in the daemon-served browser session.
   **Expected:** `runs-list-view` renders and shows seeded run rows.

2. Open a run detail route either from the runs list or the review-related run link.
   **Expected:** `run-detail-view` renders with status and snapshot data.

3. Confirm the stream status is visible on run detail.
   **Expected:** `run-detail-stream-status` reflects an open or reconnecting live-watch state rather than remaining blank.

4. Run the supporting route integration suite.
   **Expected:** Run list filtering, detail snapshot loading, and stream bootstrap all pass through the typed daemon contract.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Run list filter | Change status filter to `active` | List re-queries with the selected filter |
| Empty list | No runs returned | Empty-state branch renders |
| Detail load error | Snapshot fetch fails | Explicit error state renders |
| Terminal run | Completed/canceled run | Run detail still loads and reflects terminal status |

### Related Test Cases

- `TC-FUNC-005`
- `TC-INT-003`
- `TC-INT-004`

### Traceability

- TechSpec: "Streaming Contract"
- Task references: `task_10`, `task_14`
- ADR-005: run detail/live-watch browser coverage is part of the core validation bar

### Notes

- This is the baseline live-watch case. Overflow and reconnect semantics are intentionally split into `TC-INT-003`.
