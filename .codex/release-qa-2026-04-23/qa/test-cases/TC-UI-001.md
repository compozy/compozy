## TC-UI-001: Daemon-Served Web UI Browser Smoke

**Priority:** P0 (Critical)
**Type:** UI/Visual
**Status:** Not Run
**Estimated Time:** 30 minutes
**Created:** 2026-04-23
**Last Updated:** 2026-04-23
**Automation Target:** E2E
**Automation Status:** Existing
**Automation Command/Spec:** `bun run --cwd web test:e2e`, Browser Use or `agent-browser` manual evidence
**Automation Notes:** Existing Playwright covers dashboard/workflows/spec/memory/reviews/runs/archive/start. Browser validation provides local visual/runtime evidence.

### Objective

Verify the daemon-served Web UI loads in a real browser, core navigation works, and visible state matches the seeded daemon workspace.

### Preconditions

- Daemon HTTP listener is running with Web UI assets or dev proxy configured.
- Browser tooling is available.

### Test Steps

1. Open the daemon Web UI URL in the browser.
   **Expected:** Dashboard renders, active workspace is visible, and no blank page appears.
2. Navigate to workflows.
   **Expected:** Workflow inventory renders.
3. Open a workflow task board and a task detail page.
   **Expected:** Task board and task detail views render.
4. Navigate to runs/reviews/memory.
   **Expected:** Each route renders the expected list/detail/empty state without console errors.
5. Capture screenshots.
   **Expected:** Screenshots are saved under `qa/screenshots/`.

### Edge Cases

| Variation     | Input                    | Expected Result                                       |
| ------------- | ------------------------ | ----------------------------------------------------- |
| Invalid route | `/does-not-exist`        | Not-found/error state renders, not a white screen     |
| API loading   | Slow initial daemon read | Loading state resolves to dashboard or explicit error |
