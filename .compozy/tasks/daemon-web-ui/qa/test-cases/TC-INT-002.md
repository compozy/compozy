## TC-INT-002: Memory Index, Opaque File IDs, and Stale Recovery

**Priority:** P1 (High)
**Type:** Integration
**Status:** Not Run
**Estimated Time:** 12 minutes
**Created:** 2026-04-21
**Last Updated:** 2026-04-21
**Automation Target:** Integration
**Automation Status:** Existing
**Automation Command/Spec:**
- `bunx vitest run --config web/vitest.config.ts web/src/routes/-spec-memory-flow.integration.test.tsx web/src/systems/memory/adapters/memory-api.test.ts web/src/systems/memory/components/workflow-memory-view.test.tsx`
**Automation Notes:** The route flow proves memory index/detail navigation, opaque `file_id` fetches, stale-workspace handling, and document-level errors without exposing filesystem paths to the browser.

### Objective

Verify that the browser consumes memory documents through daemon-issued opaque `file_id` values, handles stale-workspace failures cleanly, and keeps index/detail rendering aligned with the typed contract.

### Preconditions

- [ ] Memory index/detail fixtures are available.
- [ ] The active workspace is selected.
- [ ] The memory route remains read-only in v1.

### Test Steps

1. Run the focused memory suite listed above.
   **Expected:** The vitest run exits `0` and all listed specs pass.

2. Confirm the memory index route renders workflow cards/entries.
   **Expected:** The index shows workflows and detail links without exposing raw repo paths in the route itself.

3. Confirm the detail route loads the first memory file and can switch to another file by opaque `file_id`.
   **Expected:** The document body updates, and the request URL uses `file_id` rather than `display_path`.

4. Confirm stale-workspace and missing-file branches.
   **Expected:** `412 workspace_context_stale` and `404 memory_file_not_found` both surface explicit UI errors.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Initial detail load | First memory entry available | Document body renders |
| Alternate file selection | Click a different entry | Request uses opaque `file_id` and content updates |
| Stale workspace | Memory index returns `412` | Explicit load error renders |
| Missing document | Selected file returns `404` | Document-error state renders |

### Related Test Cases

- `TC-FUNC-006`
- `TC-INT-005`

### Traceability

- TechSpec: "Document Read and Cache Strategy"
- Task reference: `task_12`
- Shared memory note: browser memory/document transport stays on daemon-issued typed payloads and opaque `file_id` values

### Notes

- This case deliberately covers both index and detail because Playwright smoke currently deep-links only into the detail route.
