# BUG-20260713-goal-judge-unconstrained-leaks-session: Goal judges do unrelated work and remain active

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Lea
- **Journey Step:** J-26, report a completed Goal and wait for its verdict
- **Scenarios:** GL-judge-session-contract; GL-004
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** CH-046 live Goal lifecycle

## Summary

The command judge for a session-origin Goal runs as an unrestricted `general` Cursor system session with the normal tool surface. The first two judge attempts ignored the exact-one-JSON-object contract, made 90 and 68 unrelated tool calls, returned no verdict JSON, and remained ACTIVE after their criterion finished. A later Goal created a third judge that also remained ACTIVE. Real Goal work is repeatedly rejected as `judge_output_malformed`, provider cost grows, and temporary judge sessions leak into the operator inventory.

## Reproduction

1. In `CH-046` / Feature Tour, start a valid Goal from live Cursor/Grok session `sess-7842125cce618d86`.
2. Let the work turn report `complete` with a durable evidence reference.
3. Observe the agent command-judge session and its transcript/tool activity.
4. Repeat once after Resume and inspect the agent live-session list after both criteria finish.

**Expected:** Each judge attempt has a verdict-only contract, no unrelated tool capability, one schema-valid JSON result or one bounded typed failure, and mandatory temporary-session cleanup on every terminal path.
**Actual:** Both judge sessions performed broad workspace/tool work, produced malformed output, and remained system/ACTIVE.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/goal-lifecycle-blocked-after-three-turns.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/goal-judge-sessions-remain-active.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/evidence/investigate-judge-malformed-turn1.json`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/screenshots/goal-judge-clear-residual.dom.txt`
- Goals `looprun-5a1acf5934fef596` and `looprun-1667f72b7cdb7128`; active judge sessions `sess-37f86bd295697c71`, `sess-f49398af9db4c77d`, and `sess-14af1951acb1bcae`.

## Fix

- **Root cause:** The judge runner inherited the agent profile's `approve-all` policy, accepted JSON extracted from prose or fences, and never owned the temporary session's terminal cleanup. A completed criterion could therefore perform broad work, return non-contract output, and leave the system session ACTIVE.
- **Fix commit:** uncommitted QA remediation batch
- **Regression test:** The canonical daemon Goal E2E preserves the three-turn `rejected`, `rejected`, `approved` invariant, proves one stopped judge session per attempt, and proves restart replay creates no additional judge. The runner/gate suites cover deny-all, strict whole-object JSON, and cleanup on success, malformed output, create/prompt failure, cancellation, timeout, and cleanup failure.

## Verification

- Automated production integration is green under race, including three stopped temporary judges and stable IDs after restart.
- Fresh real Cursor/Grok 4.5 replay passed. Goal `looprun-a6a4368bf1fc8c49` completed in one turn; judge `sess-284fdef67433e103` emitted one strict JSON verdict, performed no tool calls, stopped, and left no active temporary system session. The Run settled `done` with authoritative `{"status":"complete"}` output.
- A second active-judge replay cleared Goal `looprun-d8466636e525f1e5`; temporary judge `sess-3e07f85d0d2ac987` stopped with `user_canceled`, no system session remained active, and no successor generation was admitted.
- Evidence: `/Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/network/catalog-global-goal-acceptance.json`, `qa/screenshots/catalog-global-goal-approved.png`, and `qa/screenshots/goal-clear-during-judge-joined.png` in the same lab.

## Re-found (2026-07-13)

Fresh post-fix Browser replay created Goal `looprun-e6830bc6fd4a086f` from real Cursor/Grok 4.5. The first two work turns satisfied the authored text criterion, but both were rejected with `judge_output_malformed`. Judge session `sess-9e7301e91c86f599` was correctly classified `system`, used channel `goal_judge`, and eventually stopped, yet its persisted audit contains 60 `tool_call` events, 30 `tool_result` events, 105 thoughts, and only malformed agent output. Ask mode plus the intended empty policy therefore did not constrain the real Cursor provider to a verdict-only capability boundary. The cleanup half improved, but the central authority/output contract remains broken and this bug is reopened.

The final remediation moved verdict authority to `agent_message` only, rejects any judge tool activity, removes provider tool servers for verdict-only sessions, and concatenates streamed message chunks byte-for-byte before strict whole-object JSON parsing. The final real-provider replay above verifies the corrected capability, output, and cleanup boundaries.
