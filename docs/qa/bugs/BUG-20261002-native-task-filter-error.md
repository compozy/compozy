# BUG-20261002-native-task-filter-error: Invalid task filters are reported as backend failures

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Ada
- **Journey Step:** J-operate-workspace-context, inspect the project's pending work
- **Scenarios:** ET-native-workspace-scope-isolation
- **Found:** 2026-10-02 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction and evidence

The hosted operations session `sess-5ce7fb25138d53ed` supplies `status: "queued"` to
`compozy__task_list`. Queued is a run status, not a task status. The call returns
`tool_backend_failed` instead of identifying the invalid filter. The agent recovers by listing
tasks without that filter and inspecting runs; the misleading error makes recovery harder.
Evidence: `native-scope-final-history-operations.json` (call sequence 151, result 154) and
`native-scope-result-operations.json` in `docs/qa/evidence/2026-10-02-untested/`.

## Fix

The native task-list adapter forwards the task service's typed validation error without using
the existing task error mapper. Generic dispatch therefore reports a backend failure. Apply
`nativeTaskToolError` at the adapter, matching the other native task handlers. No schema,
accepted status, persistence or permission changes are needed.

Invariant: task catalog validation errors retain `tool_invalid_input` / `schema_invalid` at
native dispatch. Owning layer and canonical suite: daemon adapter, `TestDaemonNativeTools` in
`internal/daemon/native_tools_test.go`. The regression uses the real catalog normalizer at the
service I/O boundary and fails before the production edit: `task-filter-red-runtime.log`.
The earlier `task-filter-red.log` is a corrected test compilation mistake, not behavioral proof.

- **Fix commit:** pending
- **Retest:** passed for the corrected error classification

## Verified replay

The rebuilt isolated daemon returns `tool_invalid_input` / `schema_invalid` for the same invalid
filter through native CLI, HTTP and UDS (both transports return 400). Removing the filter returns
the same three persisted task IDs through a fresh native read and the independent direct task CLI.
This is an Ada operator replay of the shared native dispatch; it does not claim a new hosted-agent
run. Evidence: `task-filter-replay-*`, `task-filter-invalid*`, `task-filter-recovered.json`,
`task-filter-independent-list.json`, and `task-filter-fresh-list.json`. The complete canonical native
race suite passes in `task-filter-green.log` (25.888s); the test-shape checker is clean.

All affected `make gate` lanes pass; receipt: `task-filter-gate.log`.
