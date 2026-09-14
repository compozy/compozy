# BUG-20260914-mcp-invalid-definition-repair: An invalid MCP credential prevents its own repair

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** Major
- **Persona Affected:** Bruno
- **Journey Step:** J-mcp-authorize-repair, replace an already-invalid credential binding
- **Scenarios:** ET-cli-mcp-authorize
- **Report:** docs/qa/reports/2026-09-14-marketplace-review-public.md

## Reproduction

After a prior invalid credential replacement, submit a valid Settings PUT with a new owned client secret for editorial/writer-cloud. The API returns 500 before replacing the definition. Evidence: `docs/qa/evidence/2026-09-14-marketplace-review-public/step-125.json`.

## Fix

The Settings apply coordinator used the fully enriched collection to determine whether an MCP already existed. Resolving the old credential status failed before the correction could be submitted. Existence now uses the existing owner-qualified definition resolver, which does not probe authentication or runtime health. The replacement still goes through normal definition validation and credential ownership checks.

The existing `TestConfigApplyServiceRecordsLiveApplyAndAdvancesGeneration` suite covers repair and creation while prior auth status fails, persisted read-back, and unchanged replacement/addition lifecycle classification. Together with the existing MCP ownership and runtime suites, focused race checks passed in 6.417s. Both changed test suites pass the test-shape checker. Public replay is pending.
