---
id: LP-editor-authoring-walk
area: LP
title: Author the Spec 1 failure contract in the loop editor and watch the run honour it
persona: Bruno
journey: J-04
expected: A custom Loop opens in the editor with every Spec 1 key authorable at its real DSL path — `deadline`, `retry` (`max_attempts`, `backoff.base`/`.max`, `non_retryable`), `result_contract`, and `on_error` as route XOR allow_fail plus its own effect list. The six node triggers (`on_retry`, `on_success`, `on_pause`, `on_timeout`, `on_cancel`, `on_quarantine`) and the seven contract terminals (`on_done`, `on_noop`, `on_blocked`, `on_failed`, `on_exhausted`, `on_stalled`, `on_canceled`) render as plain effect lists whose entries are emit XOR tool by construction; an unauthored list shows no zero badge and publishes no empty key. A wait node declares exactly one of `for`/`until`/`event` — switching mode clears the other two in the same edit — plus `expect`, `ahead_arrival`, and the `expires` fold; a run-loop node declares `on_parent_close`. Every diagnostic is daemon truth: errors disable Publish with their own code, warnings (including `wait_expiry_without_path`) stay visible and never gate, and a clean verdict renders no counter at all. A read-only source (marketplace/user/additional) is immutable through every path — palette, contract, inspector, connect, delete — with Publish disabled, Validate available, and Fork the one legal verb; forking keeps the same name and route and flips the editor to editable workspace truth. A publish the daemon rejects with 422 shows a danger strip listing the returned issues while the version pill holds. Start-binding allowlist authoring does not ship (ADR-018).
entry_points: web /loops/:name/editor; PATCH /api/workspaces/:ws/loops/:name; POST /api/workspaces/:ws/loops/:name/validate; POST /api/workspaces/:ws/loops (fork_from_name); GET /api/workspaces/:ws/loops/:name
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-operator-lifecycle-ui;LP-error-route-fallback;LP-on-error-notification-with-context;LP-transient-blip-heals
---

story: As a loop author I declare how a step should fail — how often it retries, whether the error is absorbed or routed, who gets told — and publish it, without leaving the editor or typing a key the engine does not model.

design: docs/design/opendesign/loops/loop-editor.html + loop-editor-states.html

truthful-ui: every authorable key traces to `internal/loop/dsl` (`NodeLifecycleState`, `TriggerEffects`, `ContractLifecycleState`, `WaitParams`), and every verdict to the shared Go linter surfaced through `POST /validate` or the publish 422 — the editor never computes an invariant. Controls appear only where the runtime executes them: retry and deadline on action nodes (a control node produces no task run), `result_contract` only where an output schema exists, `on_parent_close` only on run-loop. Read-only is a capability that closes each mutation path, not a strip over live controls (SD-007).

evidence-seed: visual-contract bundles at .compozy/tasks/loop-node-lifecycle/evidence/visual/task_09/vc-e1..vc-e8 (dirty custom, node reliability, contract terminals, wait, run-loop parent close, lint severity split, read-only source, publish rejected); Vitest WT-005..WT-008; Playwright E2E-016.

qa-plan: Task 13 walks this in a browser against an isolated live lab: author the envelope on a real custom Loop, publish, start a run, and confirm the run page reflects the authored contract. Probe the read-only fork path and a real 422 rather than a mocked one.

src: .compozy/tasks/loop-node-lifecycle/task_09.md
