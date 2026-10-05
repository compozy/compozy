# BUG-20260713-loop-failure-hidden: A stalled Loop hides the action failure that the operator must fix

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Lea
- **Journey Step:** J-01 arrive and use run, step 6
- **Scenarios:** LP-action-failure-detail;LP-046
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
**Actual:** The UI shows `Failed` twice and `Stalled`; the persisted node output is only `loop_action_failed`, while the daemon log reduces the failure to `tool \"ext__spec_cycle__import_tasks\" backend failed`.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-001-software-delivery-stalled-missing-taskset.png`
- `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/compozyqa-108e1613c829/runtime/logs/compozy.log` lines 1268-1277.
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
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-001-loop-failure-detail-fixed.dom.txt`.
- **Result:** Verified. The terminal state remains truthful while the operator now receives the missing prerequisite and recovery path.

## Regressed — 2026-10-05, managed worker policy refusal

Bruno's CH-026 replay now correctly refuses a node tool outside its Agent allowance.
Run looprun-764eae115b4ac19d ends Exhausted at its one-round cap with no worker session,
but CLI Loop status/why and HTTP/UDS output_ref contain only loop_action_failed and
The action failed before producing an output. The failed node supplies no policy reason.
Following Open record or reading task run show reveals the deterministic underlying
allowed_tools override tool compozy__config_get widens agent profile error.
This is the same lost-action-cause symptom at a newly reached boundary; the earlier
extension failure repair remains intact.

Evidence: docs/qa/evidence/2026-10-02-untested/loops-policy-replay-bruno-
widening-{read,terminal,why-correct,task-run,reloaded}.json,
failed-{node,record}.json and widening-record.png. The 16-frame recording at
/Users/pedronauck/.config/browser-harness/agent-workspace/recordings/loops-policy-replay-bruno
is closed, and the failure-record screenshot was inspected.

Root cause: the session subset validator returns only formatted ErrValidation, so the
operator-safe Loop failure mapper cannot distinguish this known policy refusal from an
arbitrary unsafe runtime error. Add a typed policy violation at the validator and translate
only that type into bounded cause/recovery, preserving the existing raw CLI error and
ErrValidation identity. Never expose arbitrary underlying error strings.

The managed runtime integration suite already owns real widening rejection. Extend that
case to require the policy refusal's safe Loop projection, and reuse the existing session
subset and daemon failure-metadata suites as canaries. No new test file, schema or wire shape.
Repair and original-persona replay remain pending.

### Policy-boundary repair replay — 2026-10-05

The extended managed-runtime integration assertion failed with generic loop_action_failed,
then passed after the session validator retained a typed policy error and the daemon mapped
that exact type to allowed_tools_policy_violation. The raw validation error and ErrValidation
identity remain intact; arbitrary error text is not exposed.

Fresh Bruno run looprun-4e3ef95a3c021763 publishes the refused config_get tool and recovery
guidance through Loop status and both transports. Web Details renders those same values
after reload; the screenshot loops-policy-safe-bruno-reloaded-failure.png was inspected.
Its adjacent valid run looprun-5e8b200a211f5041 completes with the one-tool policy. Receipts:
docs/qa/evidence/2026-10-02-untested/loops-policy-safe-bruno-*.
The exact recording loops-policy-safe-bruno is closed (11 frames). The owning integration
and unchanged safe-failure/session subset suites pass. Gate/commit provenance remains pending.
