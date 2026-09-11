# BUG-20260911-memory-health-workspace-not-found: Missing workspace health appears as a server outage

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Rafa
- **Journey Step:** J-digest-sessions-into-memory, inspect an unavailable workspace
- **Scenarios:** MS-011
- **Found:** 2026-09-11
- **Report:** docs/qa/reports/2026-09-10-qa-execution-unblock.md

## Reproduction

Request GET /api/memory/health?workspace_id=missing-health-workspace through the isolated HTTP and UDS surfaces. Both return500 memory.internal. HTTP only reports Internal Server Error; UDS reports workspace not found. A fresh HTTP retry repeats500. Valid workspace health remains ok. Evidence: health-http-invalid.json, health-uds-invalid.json, health-http-invalid-retry.json in the report evidence directory.

## Cause and bounded repair

StatusForMemoryError recognizes filesystem absence but omits the workspace resolver's ErrWorkspaceNotFound sentinel. A routine missing target therefore falls through to500 and HTTP internal-error masking. Classify the wrapped workspace sentinel as404 using the existing memory.not_found payload path. The existing canonical TestMemoryHandlersAndHelpers covers both transport masking policies with a resolver I/O stub and real memory store. No schema or permission change.

## Cross-surface impact

HTTP/UDS memory handlers and CLI consumers retain existing error shapes, with the missing workspace correctly classified as404 and the actionable diagnostic retained. Other memory handlers using this classifier receive the same correction. Native tools retain their separate error mapping; hooks/config/SDKs and persisted workspace data are unchanged. Web Memory settings has no new control. Official skill guidance and site documentation already describe workspace-scoped health; no new public field or documentation contract is introduced. No migration is required. Real retest and delivery gate passed.

## Retest

The owning regression reproduced500 with and without HTTP masking, then the full TestMemoryHandlersAndHelpers passed with race detection. Test-shape checker passed. Rebuilt daemon6228 returns identical404 memory.not_found and diagnostic for HTTP/UDS missing workspace requests. Valid CLI/HTTP/UDS health payloads agree exactly after removing CLI resolution metadata. Fresh browser navigation to Settings Memory displays the global count and disabled Dream control after restart. `health-retest-proof.json` links the proof. Required gate passed on retry. The first run had an unrelated catalog TempDir cleanup failure; three targeted repetitions and the full gate retry passed without altering that test.
