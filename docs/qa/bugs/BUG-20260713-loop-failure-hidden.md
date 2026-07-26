# BUG-20260713-loop-failure-hidden: A stalled Loop hides the action failure that the operator must fix

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Lea
- **Journey Step:** J-01 arrive and use run, step 6
- **Scenarios:** LP-action-failure-detail
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** n/a

## Summary

Lea started the bundled `software-delivery` Loop with a slug that had no matching task files. The run truthfully ended as `Stalled`, but both failed `load_tasks` attempts exposed only `Failed` and the opaque `loop_action_failed` reference. The run detail never showed which path was searched, the backend error, or a recovery step, so Lea could not tell whether the Loop, extension, provider, or her input was broken.

## Reproduction

- **Charter:** CH-001 · **Tour:** Feature Tour
- **Environment:** laptop / wifi-fast / en-US; isolated daemon at `http://127.0.0.1:58941`; in-app browser.

1. Finish onboarding with the isolated workspace and Cursor `cursor-grok-4.5-high` as the default runtime.
2. Open Loops, select the bundled `software-delivery` definition, and choose Run Loop.
3. Enter `helix-v1-launch` for `slug` when the workspace has no `.compozy/tasks/helix-v1-launch/task_*.md` files.
4. Start the run and wait for its terminal state.
5. Inspect the expanded failed generation and the live event rail.
6. Independently read the persisted run through `GET /api/workspaces/:workspace_id/loop-runs/:run_id`.

**Expected:** The run detail preserves the backend failure reason for `load_tasks`, identifies the missing or unmatched task pattern, and offers enough recovery guidance to correct the input before retrying.
**Actual:** The UI shows `Failed` twice and `Stalled`; the persisted node output is only `loop_action_failed`, while the daemon log reduces the failure to `tool \"ext__dev_cycle__import_tasks\" backend failed`.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-001-software-delivery-stalled-missing-taskset.png`
- `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-108e1613c829/runtime/logs/agh.log` lines 1268-1277.
- Persisted run `looprun-2cf0340ae8091bbe` in workspace `ws_06366aad69887872` returns `output_ref: \"loop_action_failed\"` for both failed `load_tasks` generations.

## Fix

- **Root cause:** The bundled extension returned an unstructured JSON-RPC error, the extension host collapsed it into a generic backend failure, and Loop failure persistence projected only the `loop_action_failed` reason code into the node `output_ref`. The Web timeline therefore had no operator-safe detail to render.
- **Fix:** The extension now emits a typed operator-safe `ToolError`; the host restores that envelope, the daemon redacts and bounds the cause/recovery text, globaldb durably projects the structured `action_failure` payload, and the Loop timeline renders it in a danger alert beneath the failed node.
- **Fix commit:** pending final task commit.
- **Regression test:** Canonical extension-runtime, Loop failure metadata, globaldb claim-terminal projection, and Web Loop run-page suites cover the structured error from RPC restoration through durable UI rendering.

## Verification

- **Retested:** 2026-07-13 in the same isolated lab after rebuilding and explicitly restarting the registered daemon process.
- **Automated evidence:** `gofmt -d` returned no diff; `go test -race ./internal/daemon -count=1` passed 1,159 tests; `go test -race ./internal/store/globaldb -count=1` passed 666 tests. The worker's affected Web lane passed codegen-check, typecheck, 3,319 tests in 391 files, and focused lint.
- **Public evidence:** Browser-created run `looprun-b165c15b174e3d40` rendered `No task set matched .compozy/tasks/helix-v1-launch/task_*.md.` and `Create the matching task set or correct the Loop input, then retry the run.` beneath both failed `load_tasks` nodes. The public run API persisted the same typed `action_failure` payload.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-001-loop-failure-detail-fixed.dom.txt`.
- **Result:** Verified. The terminal state remains truthful while the operator now receives the missing prerequisite and recovery path.
