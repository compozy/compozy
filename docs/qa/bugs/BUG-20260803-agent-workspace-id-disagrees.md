# BUG-20260803-agent-workspace-id-disagrees: Agent commands disagree on the owning workspace ID

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-07 agent-operated run, step 5
- **Scenarios:** LP-agent-operates-lifecycle-via-native-tools; TA-076
- **Found:** 2026-08-03 · **Report:** docs/qa/reports/2026-08-03-loop-node-lifecycle.md

## Summary

Ada receives two different `workspace_id` values for the same workspace-local agent. Agent create,
info, and list return an internal durable identity, while workspace info returns the registered
`ws_…` ID used by public CLI, HTTP, and UDS routes. The matching agent name and definition digest
show that both responses describe the same object, so an operator or agent cannot safely reuse the
identifier from every public read.

## Reproduction

- **Charter:** CH-agent-loop-lifecycle-parity · **Tour:** Feature Tour
- **Environment:** isolated CLI lab / desktop / wifi-fast / en-US

1. Register a workspace with `compozy workspace add <path> --name identity-probe --json`.
2. Create `identity-probe-agent` in that workspace with `compozy agent create … --workspace <ws-id> --json`.
3. Read the same definition through `compozy agent info`, `compozy agent list`, and `compozy workspace info`.
4. Compare `workspace_id` for the matching agent name and definition digest.

**Expected:** Every public response uses the registered workspace ID `ws_f76fa2c6197d6cef`.
**Actual:** Agent create, info, and list return `01KZ44R0EG3YRV2PSJAQ593WX7`; workspace info returns `ws_f76fa2c6197d6cef`.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-agent-workspace-id-contract-20260803-154310-906856-lab/qa-artifacts/qa/evidence/agent-workspace-id-mismatch.json`
- Independent public reads: `agent info`, `agent list`, and `workspace info` returned the same agent digest with conflicting workspace IDs.
- `/Users/pedronauck/dev/qa-labs/compozy-agent-workspace-id-retest-20260803-155453-101663-lab/qa-artifacts/qa/evidence/agent-workspace-id-retest.json`
- `/Users/pedronauck/dev/qa-labs/compozy-native-agent-workspace-id-retest-20260803-160237-155319-lab/qa-artifacts/qa/evidence/native-agent-workspace-id-retest.json`

## Fix

- **Root cause:** Agent create/list/get/duplicate paths project `ResolvedWorkspace.WorkspaceID`, the durable identity file value, while workspace detail and public route ownership use `ResolvedWorkspace.ID`, the registered workspace ID.
- **Fix:** Agent create, workspace list/get/catalog, update/delete history operations, duplicate targets, and `compozy__agent_create` now project the single resolved `ResolvedWorkspace.ID`. The shared native authoring result carries that normalized ID so the adapter cannot echo an alias or the durable directory identity. The durable identity remains unchanged and internal.
- **Fix commit:** pending Task 13 checkpoint
- **Regression tests:** `internal/api/core/handlers_test.go` distinguishes registered and durable IDs across create, list/get/catalog, duplicate, and history cleanup. `internal/daemon/native_create_tools_test.go` requires `compozy__agent_create` to normalize an alias to the registered ID and exclude both alias and durable ID. Both focused tests failed before their production fixes and pass under `-race`; the complete `internal/api/core` package and `TestNativeAgentCreate` also pass.

## Verification

- **Retested:** 2026-08-03 in fresh isolated labs `agent-workspace-id-retest-20260803-155453-101663` and `native-agent-workspace-id-retest-20260803-160237-155319` after rebuilding for each production change.
- **Result:** Verified. `agent create`, `agent info`, `agent list`, `workspace info`, and public `tool invoke compozy__agent_create` all returned their registered `ws_…` ID; independent reads matched each created definition digest while distinct durable identities stayed internal. Both teardown records contain `"clean": true`.
