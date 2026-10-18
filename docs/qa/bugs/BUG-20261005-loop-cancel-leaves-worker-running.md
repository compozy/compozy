# BUG-20261005-loop-cancel-leaves-worker-running: Cancel run leaves its worker executing

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-01, cancel an active Loop and trust that its worker stops
- **Scenarios:** LP-003
- **Found:** 2026-10-05 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Bruno confirms Cancel run after an execution exceeds its time limit. The run immediately
becomes Canceled, but its managed worker remains active and continues prompting for at
least 59 seconds. The confirmation promised active sessions would stop automatically.
Bruno must separately stop the worker session to end the work.

## Reproduction

- **Charter:** CH-012 · **Tour:** Feature Tour
- **Environment:** desktop 1512×862 / DPR 2 / fast Wi-Fi / en-US / America/Los_Angeles.
- **Build:** 547027459 runtime behavior; isolated daemon PID 9029, port 50727,
  binary 42a1236d1a792a8bd9e96cc282692b672dd3417143dcfc60a70991c43a299b1a.

1. Start the provider-backed implement-tasks run described in the linked wall-limit finding.
2. In Web, choose Cancel run and confirm the dialog for looprun-4e6bb5f893b3b18f.
3. Read Loop why through CLI and the managed worker status independently through UDS.
4. After the canceled outcome is durable, verify whether the worker still has an active prompt.

**Expected:** Canceling the run settles its owned worker without requiring a separate session stop.

**Actual:** The run records canceled/operator_cancel at 09:44:55.374524Z. At 09:45:54Z worker
sess_c848f72460c729e1c69ec46cbe09bc79 still reports active, prompting, active_prompt=true.
A separate public session stop --wait succeeds with verified=true and forced escalation.
This differs from the false operator-attribution bug on already completed workers.

## Evidence

- docs/qa/evidence/2026-10-02-untested/
  loops-terminal-guardrails-bruno-{cancel-confirm-2,final-worker,canceled-why,stop-worker}.json; loops-terminal-guardrails-bruno-canceled.png.
- Recording: /Users/pedronauck/.config/browser-harness/agent-workspace/recordings/loops-terminal-guardrails-bruno
  (closed, 20 frames).
- Exact owned resources and cleanup history: loops-terminal-owned.json.

## Fix

The durable transcript records ACP prompt_stop_reason=cancelled at sequence 134, a stopped
session at sequence 136, and a new schema-repair user prompt at sequence 138. The provider
resumes at 09:44:58Z. Cleanup did stop the process; the action collector incorrectly accepted
the canceled turn as a successful response. Output validation then retried the incomplete JSON,
which caused the ordinary prompt path to reactivate the stopped session.

The adapter now maps the typed ACP canceled stop reason to the existing safe cancellation
failure and preserves context.Canceled identity and measured usage. The run-agent executor
already stops on prompt errors, so canceled output never reaches schema validation or repair.
No session restart policy, public DTO, persisted enum or configuration is changed.

Invariant: a canceled provider turn cannot become an action answer, whether its text is absent,
partial JSON or complete JSON; its recorded usage is retained. Owner: daemon prompt adapter.
Canonical suite: TestCollectLoopPromptResultProviderFailures in loop_runtime_adapters_test.go.
The three added cases fail with a nil error before the production repair and pass afterward.
The first attempt used a nonexistent constant name and only proved a compile error; the second
attempt is the behavioral red. Receipts: loops-cancel-regression-red-2.json and
loops-cancel-regression-green.json. The original-persona first-turn replay passes; the fix commit is 5216cbba009d3ae6fb693e3cf801f2ae5484c66f.

## Verification

The fresh CH-012 Bruno replay uses the same desktop, locale and network conditions with
the real Codex/GPT-6.1-Sol runtime selected from the public live catalog. Binary SHA:
d51bb3b8494bceeacc1d5672caddf3f1d1b880a6790c8468dc58662ce7cef340.
Run looprun-62035a5882bc823b is canceled in Web at 10:06:46.095085Z while its first prompt
is active. Worker sess_a131f1fc9f0c86425a3582acf123d0bb records ACP cancelled at sequence 7
and session_stopped at sequence 9, 10:06:48.278779Z. Final history contains exactly one user
prompt and one canceled provider turn. Independent UDS status confirms stopped, verified=true,
active_prompt=false; no separate session-stop action was needed. CLI why, HTTP and a fresh
Web reload retain canceled/operator_cancel. The loaded screenshot was visually inspected.

The preceding replay looprun-2632862b79577dbd also stays stopped, but it canceled an already
running schema-repair turn after the provider rejected the selected gpt-6-luna model. That
limited observation is retained and excluded from first-turn regression proof. A pre-existing
successful implement-tasks run still reads Done in Web. Its initial extra wait targeted the
wrong accessibility role and timed out; the subsequent outcome capture succeeds.

- **Regression test:** TestCollectLoopPromptResultProviderFailures (three canceled-output cases).
- **Race suites:** daemon prompt collectors and policy-gate judge, Loop run-agent/transform
  executors and action timeout all pass; see loops-cancel-owning-suites.json.
- **Replay receipts:** loops-cancel-first-turn-bruno-{prompt-0,cancel,history,history-final,
  worker-uds,worker-final,why,run-http,reload}.json.
- **Screenshot:** loops-cancel-first-turn-bruno-reloaded.png.
- **Recording:** /Users/pedronauck/.config/browser-harness/agent-workspace/recordings/loops-cancel-first-turn-bruno
  (closed, seven frames). The earlier limited replay is closed with ten frames.
- **Ownership:** loops-cancel-first-turn-owned.json and loops-cancel-replay-owned.json retain
  the terminal runs and stopped session histories for evidence.
- **Fix commit:** 5216cbba009d3ae6fb693e3cf801f2ae5484c66f. All required gate lanes are CURRENT-PASS for
  tree a565b13760f0f1b73855b2f1bad11a56830b14e1; the commit preserves that exact tree.
  See loops-cancel-delivery-gate.json, loops-cancel-gate-status.json and loops-cancel-fix-commit.json.
