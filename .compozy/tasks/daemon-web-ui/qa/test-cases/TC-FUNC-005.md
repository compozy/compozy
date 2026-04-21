## TC-FUNC-005: Reviews Index, Review Detail, and Related Run Navigation

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
- Supporting integration: `bunx vitest run --config web/vitest.config.ts web/src/routes/-reviews-flow.integration.test.tsx`
**Automation Notes:** Playwright smoke validates the daemon-served reviews index, issue-detail navigation, and related-run hop into the run detail page. The route integration suite covers empty-state, stale-workspace, missing-issue, and invalid-round branches.

### Objective

Verify that review rounds and individual issues are inspectable from the browser, and that operators can follow review-related run links into the runs console.

### Preconditions

- [ ] Seeded review data exists in the Playwright fixture.
- [ ] Active workspace resolution is healthy.
- [ ] Review route integration fixtures still match the typed daemon payload.

### Test Steps

1. Open `/reviews` in the daemon-served browser session.
   **Expected:** `reviews-index-view` renders and lists workflow review cards/issues.

2. Open a review issue from the index.
   **Expected:** `review-detail-view` renders with issue metadata and markdown content.

3. Follow the related run link from review detail.
   **Expected:** The browser navigates to `/runs/$runId` and the run detail page becomes visible.

4. Run the supporting integration suite.
   **Expected:** Empty-state, stale-workspace, missing-issue, and invalid-round paths all produce explicit UI states.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| No latest review | Dashboard workflow has no latest review | Reviews index empty state renders |
| Stale workspace | `412 workspace_context_stale` | Reviews index error is explicit |
| Missing issue | Unknown issue ID | Review detail load error renders |
| Invalid round segment | Non-numeric round in route | Invalid-round error renders immediately |

### Related Test Cases

- `TC-INT-001`
- `TC-FUNC-007`

### Traceability

- TechSpec API endpoints: `/api/reviews/:slug`, `/api/reviews/:slug/rounds/:round/issues/:issue_id`
- Task references: `task_12`, `task_14`
- ADR-005: browser coverage for reviews is part of the primary validation bar

### Notes

- This case proves the review read surface and review-to-run navigation. It does not replace the review-fix dispatch action coverage in `TC-INT-001`.
