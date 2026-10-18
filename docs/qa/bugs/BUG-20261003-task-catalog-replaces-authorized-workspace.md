# BUG-20261003-task-catalog-replaces-authorized-workspace: Editorial task reads return Studio work

- **Status:** verified
- **Fix commit:** 7a7780ae3
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-operate-workspace-context, assemble a cross-project handoff
- **Scenarios:** ET-native-workspace-scope-isolation
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

An authorized Studio agent asks for Editorial's task inventory and receives Studio tasks. The
agent notices the returned workspace IDs and reports Editorial's inventory as unavailable.
The separate Editorial memory write succeeds and persists in the correct project.

## Reproduction

- **Charter:** CH-native-workspace-handler-boundary · **Tour:** Feature Tour
- **Environment:** continuing isolated macOS lab, CLI/HTTP, real Codex hosted agent, build e127956a3.

Start an approve-all Studio session. Ask for a coordination reference in Editorial memory and a
cross-project task inventory. After all-scope access is refused, the agent calls task_list with
workspace Editorial, then its registered path. Both return the three Studio tasks. Independent
native HTTP and direct task CLI reads return Editorial's correct empty catalog.

**Expected:** an allowed explicit workspace selects that project's catalog; a denied target fails
before reading. The caller workspace is used only when the target is omitted.
**Actual:** the shared task service replaces the authorized target with the caller workspace.

## Evidence

Receipts under docs/qa/evidence/2026-10-02-untested/: native-handoff-summary.json (calls 123 and
148), native-handoff-history-4.json, native-handoff-editorial-default-scope.json,
native-handoff-editorial-explicit-scope.json, and native-handoff-editorial-task-independent.json.
The memory write/read receipts show Editorial's retained studio_coordination.md. The caller was
stopped and the persona walk ended before source investigation (native-handoff-ended.json).

## Fix

Root cause: Service.ListTaskCatalog unconditionally overwrites workspace queries for agent actors,
ignoring the existing task-resource workspace authorization policy. Preserve explicit targets,
check them through that policy, and inherit the caller only for omitted workspace input.

Invariant and owning suite: the task service catalog preserves an authorized explicit workspace,
rejects foreign access without policy, and retains caller/profile defaults. The real-SQLite catalog
integration suite in internal/task/manager_integration_test.go owns this coverage.

## Verification

The existing task resource authorizer now checks the explicit target before the service binds its
catalog. Caller identity and profile scope remain unchanged. The new real-SQLite cases fail before
repair (task-catalog-target-red.log) and pass afterward with adjacent catalog/dependency and claim
coverage (task-catalog-target-green.log, race-enabled, 4.520s). The convention checker has zero new
findings against the committed file; 17 pre-existing inline-case findings are unchanged.

Fresh hosted caller sess-363c7d60631ac759 reads Studio by name and receives its three tasks, then
reads Editorial by name and receives an empty page. Independent direct CLI and fresh UDS reads
agree on every task ID and project. The exact original task-catalog failure is corrected. The
agent's broader handoff remains incomplete because it misinterpreted Editorial memory as an agent
scope; that different request correctly returned not found and does not invalidate the actual
catalog call/result evidence. No product mutation or corrective nudge was used. The caller is stopped.

Receipts: task-catalog-replay-* in the cycle evidence directory. The first gate attempt stopped at
formatter drift in the new test; the repository formatter was applied and all affected gate lanes
passed (task-catalog-target-gate-retry.log).
