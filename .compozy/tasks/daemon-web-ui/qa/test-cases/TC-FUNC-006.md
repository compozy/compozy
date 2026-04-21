## TC-FUNC-006: Spec Deep Links and Document Tabs

**Priority:** P1 (High)
**Type:** Functional
**Status:** Not Run
**Estimated Time:** 10 minutes
**Created:** 2026-04-21
**Last Updated:** 2026-04-21
**Automation Target:** E2E
**Automation Status:** Existing
**Automation Command/Spec:**
- `bun run frontend:e2e`
- Supporting integration: `bunx vitest run --config web/vitest.config.ts web/src/routes/-spec-memory-flow.integration.test.tsx`
**Automation Notes:** Playwright smoke proves that daemon-served deep links to `/workflows/$slug/spec` work under the embedded SPA. The route integration suite proves the tab behavior and missing-document branches more directly.

### Objective

Verify that workflow spec documents are reachable through daemon-served deep links and that PRD, TechSpec, and ADR tabs render the typed document payloads correctly.

### Preconditions

- [ ] Spec documents exist for the seeded workflow.
- [ ] The daemon-served browser session is healthy.
- [ ] Generated routes and typed client are current.

### Test Steps

1. Run the Playwright smoke suite and navigate directly to `/workflows/daemon-web-ui/spec`.
   **Expected:** `workflow-spec-view` renders under the daemon HTTP listener.

2. Open the TechSpec tab.
   **Expected:** The TechSpec body renders and contains a known heading such as `Testing Approach`.

3. Run the supporting route integration suite.
   **Expected:** PRD, TechSpec, and ADR tab switching succeeds, and missing-document branches surface clear errors.

4. Confirm the route works as a deep link rather than only after client-side navigation from another page.
   **Expected:** Refreshing or directly opening the URL still resolves through the SPA fallback.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Direct deep link | `/workflows/<slug>/spec` | Page resolves under daemon-served SPA fallback |
| TechSpec tab | Click TechSpec tab | TechSpec markdown body renders |
| ADR tab | Click ADR tab | ADR list/content renders |
| Missing spec | `404 document_missing` | Explicit load error renders |

### Related Test Cases

- `TC-INT-002`
- `TC-INT-005`

### Traceability

- TechSpec: "Document Read and Cache Strategy"
- ADR-002: deep links must resolve under the daemon HTTP listener
- Task references: `task_12`, `task_14`

### Notes

- This case covers the spec route as a real browser deep link. It does not replace the mocked degraded/partial coverage tracked in `TC-INT-005`.
