# BUG-20261004-empty-task-blocks-null: Empty task block lists violate their array contract

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Dora
- **Journey Step:** J-complete-task-tree, confirm the last blocking requirement was cleared
- **Scenarios:** TA-010 (adjacent control read)
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

Create a block through the public CLI, clear it, then list open blocks. The CLI returns null;
HTTP GET /api/tasks/{id}/blocks returns {"blocks":null}. The OpenAPI 200 response requires blocks
to be an array and does not mark it nullable. Clients expecting that contract cannot iterate it.
The independent task detail correctly reports no remaining blocks.

Evidence: extension-task-full-state-replay-unblocked-list-1.json,
extension-task-full-state-replay-unblocked-read-1.json and
extension-task-empty-blocks-schema-check.json under the report's evidence directory.

## Diagnosis and owning regression

TaskBlockPayloadsFromBlocks returns nil for both nil and empty domain collections. HTTP/UDS and
the native task-block list share that mapper; the CLI preserves the daemon's response. Remove the
nil early return so the existing allocated empty collection carries the documented array shape.

Invariant: a successful empty block listing serializes an array. Owning layer: shared public task
response serialization. Canonical suite: TestTaskBlockHandlersReturnStatusAndBodies in
internal/api/httpapi/handlers_test.go. Its unit I/O stub supplies nil and empty domain inventories;
the real CLI/HTTP/UDS replay remains the final validation.

## Fix and verification

The mapper now allocates its empty result. Both nil and empty domain inventory regressions fail
before the repair and pass with the race detector after it. A fresh rebuilt daemon returns []
through CLI and blocks=[] through both HTTP and UDS. All three cleared block records remain
available with --all, and a foreign profile still receives 404. Evidence:
task-empty-block-list-{red,green}.json and task-empty-block-list-replay-ended.json.
