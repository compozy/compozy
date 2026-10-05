# BUG-20261005-loop-managed-allowed-tools-ignored: Loop workers ignore the node tool restriction

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-01, start a worker with a restricted tool policy
- **Scenarios:** LP-046
- **Found:** 2026-10-05 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

A Loop node's allowed_tools is silently discarded. Its worker inherits the Agent's full
tool allowance even when the node requests only one tool. A request outside the Agent's
allowlist also starts a worker instead of returning the documented bind error.
The Agent's ceiling remains enforced; this finding does not claim arbitrary tool access.

## Reproduction

Charter CH-026, Feature Tour; Bruno on desktop, fast Wi-Fi, en-US.

1. In Studio Operations / resume-editorial, create studio_inventory_reader with
   approve-reads and tools compozy__session_list and compozy__workspace_list.
2. Author studio-workspace-digest with a run-agent node allowing only compozy__workspace_list.
3. Run it from the public CLI, open its Web run page and inspect the worker's public status.
4. Author studio-workspace-settings-digest using the same Agent but allowing compozy__config_get.
5. Run it, then independently reread the run and worker through HTTP and UDS and reload Web.

Expected: the first worker exposes only workspace_list. The second request fails before
provider startup with the deterministic widening error. Actual: both workers start and
finish Done; both permission policies retain the Agent's original two-tool allowance.
The workspace approve-all default remains unchanged and the Agent override is approve-reads.

## Evidence

- Narrowing: looprun-02b0ec6711a965be / sess_eb875ab9f8020bcef0ccf7867aecd5bb.
- Widening: looprun-6396a315a620d348 / sess_d5d6fca1eade9139f0cccd4be549672f.
- Receipts: docs/qa/evidence/2026-10-02-untested/loops-policy-bruno-*.
- Authoring inputs: studio-workspace-digest.json, studio-workspace-settings-digest.json,
  studio-digest-limits.json in the same evidence directory.
- The seven-frame loops-policy-bruno recording is closed. Independent session status
  reads carry both tools, and the reloaded widening run remains Done with its worker ID.
- loops-policy-owned.json records the exact temporary resources for cleanup after replay.

Historical BUG-0021 and BUG-0023 prevented reaching this boundary and are distinct symptoms.
Definition/default limit precedence follows the documented order; it is not this defect.

## Fix

Root cause: baseCreateOptions omits ActionSessionBindRequest.AllowedTools, so the managed
binding invokes its policy gate with no override. Pinned-profile reuse also ignores an
explicit new tool restriction. The ephemeral binding already forwards the requested list.

The existing managed runtime integration suite owns the invariant: persist and enforce
the requested subset, reject widening before materialization, and refuse reuse when an
explicit restriction differs from the pinned creation policy. Use its real Manager and
SQLite fixture, with the existing ACP I/O driver; no new test file or public shape.
This is a contained contract repair within the authorized QA fix scope.

Fix commit: pending. Regression test: internal/daemon/loop_goal_managed_runtime_integration_test.go,
TestLoopGoalManagedRuntimeIntegration.

## Verification

Original public reproduction confirmed. Regression, repair and original-persona replay pending.

2026-10-05 repair replay passed in the working tree. The three integration regressions
failed before and passed after forwarding the override and revalidating explicit pinned
restrictions. The complete managed-runtime/ephemeral gate suites pass with race detection.
Bruno's fresh subset run looprun-d9f208b4d5c597ef and final run looprun-5e8b200a211f5041
both finish Done with workspace_list as their worker's only tool; public history confirms
the hosted call succeeds. The no-override sibling looprun-49da371fbfefa00c retains both
Agent tools. Both widening replays create no session and zero usage; the final one,
looprun-4e3ef95a3c021763, preserves the specific policy violation in Loop output.
HTTP/UDS and refreshed Web agree. Receipt families: loops-policy-replay-bruno-* and
loops-policy-safe-bruno-*. The recordings are closed (16 and 11 frames).
Commit provenance and the delivery gate remain pending before registry closure.

### Policy repair closure — 2026-10-05

Fix commit: 547027459508f5e2d550dcd80d5378e6ea077db3. The fourth make gate passes for frozen tree
5c7114683a5f2f718625c34e6b1bfec04c6c0f96; all required gate-status records are CURRENT-PASS.
The ordinary commit and its formatting hooks preserve that exact tree. Original Bruno
replays, independent transports and refresh proof above verify the policy boundary.
Receipts: loops-policy-delivery-gate-4.json, loops-policy-frozen-gate-status.json,
and loops-policy-fix-commit.json. Historical provider quota/OAuth legs remain unverified.
