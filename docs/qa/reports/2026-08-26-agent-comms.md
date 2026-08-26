# QA Run Report — 2026-08-26 — agent-comms

- **Scope:** Typed agent calls, durable mailbox, subagent roster, lifecycle controls, profile isolation, public runtime surfaces, Agents app, docs, and the Loop contract canary.
- **Cadence tier:** full
- **Build:** `921b6e584e99f226803750a979fbce038c132a9d` · **Environment:** fresh isolated lab pending bootstrap; native provider and browser parity will be recorded from its manifest
- **Started:** 2026-08-26T06:50:12Z · **Status:** in-progress

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Fresh isolated agent-comms lab | desktop / wifi-fast / en-US | delivery, in-session, operator-fence |
| Bruno | Fresh isolated agent-comms lab | desktop / wifi-fast / en-US | mailbox, settlement, spawn crossings, task contract, Loop canary |
| Dora | Fresh isolated agent-comms lab | desktop / wifi-fast / en-US | containment, sanitization, scope |
| Lea | Fresh isolated agent-comms lab | laptop / wifi-fast / en-US | roster and docs truth |

## Flows in Scope

- `J-delegate-work-to-an-agent` — create, settle, await, cancel, follow up, and deliver typed results.
- `J-message-a-running-agent` — send durable lineage messages and observe transport receipts.
- `J-build-a-subagent-roster` — discover described agents and call one through public surfaces.
- `J-contain-and-audit-delegation` — enforce limits, sanitization, ownership, hooks, and deleted surfaces.
- `J-supervise-delegation-trees` — inspect and control calls through the Agents app and session context.
- `J-cross-workspace-access` — preserve the consent boundary after the public spawn hard cut.
- `J-contract-a-task-result` — pin and enforce task result contracts and budgets.
- `J-complete-partial-loop` — prove Loop output contracts retained their behavior without creating calls.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-agent-comms-settlement-authority | J-delegate-work-to-an-agent / RT-agent-call-golden-path | Bruno | Interrupt Tour | Fixed, retest pending | BUG-20260826-operator-caller-model-runtime | pending QA remediation commit |
| 2 | CH-agent-comms-settlement-authority | J-delegate-work-to-an-agent / RT-agent-call-cancel | Bruno | Interrupt Tour | Pending | | |
| 3 | CH-agent-comms-settlement-authority | J-delegate-work-to-an-agent / RT-agent-call-deadline-timeout | Bruno | Interrupt Tour | Fixed, retest pending | BUG-20260826-call-deadline-activation-fence; BUG-20260826-bounded-wait-client-timeout | pending QA remediation commits |
| 4 | CH-agent-comms-settlement-authority | J-delegate-work-to-an-agent / RT-call-return-contract-repair | Bruno | Interrupt Tour | Fixed, retest pending | BUG-20260826-call-child-tool-policy | pending QA remediation commit |
| 5 | CH-agent-comms-delivery-exactly-once | J-delegate-work-to-an-agent / RT-call-wake-delivery-exactly-once | Ada | Multi-Tab Tour | Pending | | |
| 6 | CH-agent-comms-delivery-exactly-once | J-delegate-work-to-an-agent / RT-agent-call-follow-up | Ada | Multi-Tab Tour | Pending | | |
| 7 | CH-agent-comms-delivery-exactly-once | J-delegate-work-to-an-agent / RT-agent-call-batch | Ada | Multi-Tab Tour | Pending | | |
| 8 | CH-agent-comms-mailbox-backpressure | J-message-a-running-agent / RT-agent-mailbox-send-list | Bruno | Garbage Tour | Pending | | |
| 9 | CH-agent-comms-mailbox-backpressure | J-message-a-running-agent / RT-message-limits-typed-rejections | Bruno | Garbage Tour | Pending | | |
| 10 | CH-agent-comms-mailbox-backpressure | J-message-a-running-agent / RT-parked-child-idle-ttl | Bruno | Garbage Tour | Pending | | |
| 11 | CH-agent-comms-containment-fence | J-contain-and-audit-delegation / RT-delegation-depth-and-caps | Dora | Error Guessing Tour | Pending | | |
| 12 | CH-agent-comms-containment-fence | J-contain-and-audit-delegation / RT-calls-config-effects | Dora | Error Guessing Tour | Pending | | |
| 13 | CH-agent-comms-containment-fence | J-contain-and-audit-delegation / RT-session-spawn-removed | Dora | Error Guessing Tour | Pending | | |
| 14 | CH-agent-comms-sanitize-and-scope | J-contain-and-audit-delegation / RT-call-payload-sanitize-sweep | Dora | Garbage Tour | Pending | | |
| 15 | CH-agent-comms-sanitize-and-scope | J-contain-and-audit-delegation / RT-call-profile-scope-isolation | Dora | Garbage Tour | Pending | | |
| 16 | CH-agent-comms-sanitize-and-scope | J-contain-and-audit-delegation / ET-call-hooks-host-api-reads | Dora | Garbage Tour | Pending | | |
| 17 | CH-agent-comms-operator-fence | J-supervise-delegation-trees / RT-delegation-activity-tree | Ada | Interrupt Tour | Pending | | |
| 18 | CH-agent-comms-operator-fence | J-supervise-delegation-trees / RT-call-record-terminal-states | Ada | Interrupt Tour | Pending | | |
| 19 | CH-agent-comms-operator-fence | J-supervise-delegation-trees / RT-delegation-attention-signals | Ada | Interrupt Tour | Pending | | |
| 20 | CH-agent-comms-operator-fence | J-supervise-delegation-trees / RT-session-stop-subtree | Ada | Interrupt Tour | Pending | | |
| 21 | CH-agent-comms-in-session-truth | J-supervise-delegation-trees / RT-in-context-call-messages | Théo | Feature Tour | Pending | | |
| 22 | CH-agent-comms-in-session-truth | J-supervise-delegation-trees / RT-session-calls-inspector-panel | Théo | Feature Tour | Pending | | |
| 23 | CH-agent-comms-in-session-truth | J-supervise-delegation-trees / NB-agent-call-publish | Théo | Feature Tour | Pending | | |
| 24 | CH-agent-comms-roster-and-docs-truth | J-build-a-subagent-roster / SITE-agent-comms-docs-area | Lea | Feature Tour | Pending | | |
| 25 | CH-agent-comms-roster-and-docs-truth | J-build-a-subagent-roster / RT-subagent-roster-injection | Lea | Feature Tour | Pending | | |
| 26 | CH-agent-comms-roster-and-docs-truth | J-build-a-subagent-roster / RT-agent-roster-call-compose | Lea | Feature Tour | Pending | | |
| 27 | CH-agent-comms-spawn-hard-cut-crossings | J-cross-workspace-access / ET-workspace-access-mode-matrix | Bruno | Back-Button Tour | Pending | | |
| 28 | CH-agent-comms-spawn-hard-cut-crossings | J-cross-workspace-access / ET-workspace-access-prompt-outcomes | Bruno | Back-Button Tour | Pending | | |
| 29 | CH-agent-comms-spawn-hard-cut-crossings | J-cross-workspace-access / RT-session-parent-provenance | Bruno | Back-Button Tour | Pending | | |
| 30 | CH-agent-comms-task-result-contract | J-contract-a-task-result / TA-task-result-contract | Bruno | Data Tour | Pending | | |
| 31 | CH-agent-comms-task-result-contract | J-contract-a-task-result / TA-task-result-default-budget | Bruno | Data Tour | Pending | | |
| 32 | CH-agent-comms-loop-contract-canary | J-complete-partial-loop / LP-loop-contract-regime-adoption | Bruno | Feature Tour | Pending | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

Pending execution.

## What Was Fixed

- `BUG-20260826-call-deadline-activation-fence` — a deadline could win between child creation and
  activation binding, leaving the durable call correctly timed out while Create leaked internal
  fence and claim-token errors. The service now cleans up the unbound child and returns the durable
  terminal record without attempting a second claim release. Focused race-enabled regression is
  green; public CLI retest remains pending a rebuilt isolated daemon.
- `BUG-20260826-operator-caller-model-runtime` — completion delivery treated the operator caller as
  a normal agent session and attached a model runtime, contradicting the accepted spec and blocking
  later calls after crash recovery. The daemon now resolves this role before delivery and records
  operator attention without status, resume, or prompt operations. Focused regression is green;
  public restart/reuse retest remains pending.
- `BUG-20260826-bounded-wait-client-timeout` — valid call/session waits longer than 30 seconds were
  interrupted by the generic CLI HTTP timeout. Bounded waits now use the dedicated transport with
  the requested/clamped server wait plus response grace. Five focused race-enabled client tests pass;
  public UDS retest remains pending.
- `BUG-20260826-call-child-tool-policy` — unrestricted logical callers persisted an empty concrete
  tool policy, and omitted call narrowing categories did not inherit from the caller. Call children
  consequently saw `compozy__call_return` as denied and could not settle their work. Root sessions
  now materialize the native tool universe and calls inherit omitted categories. Canonical focused
  and package race suites pass; public child-catalog and settlement retest remain pending.

## Paper Cuts

None recorded yet.

## Runtime Errors Observed

- Before remediation, the short-deadline call `call-a2dd5c68719b0a90` returned joined activation
  fence and invalid-claim diagnostics even though its durable record was already `timeout` with
  `call_timeout`. See the linked bug and isolated-lab evidence.
- The operator caller `ses_operator_225d12b20945a1568f3fee27` acquired a model runtime through
  completion delivery. After interrupted daemon shutdown, its dead runtime blocked the next call
  before admission. See `BUG-20260826-operator-caller-model-runtime`.
- `session wait --timeout 55s` and `call await --timeout 60s` both failed at about 30 seconds with
  `Client.Timeout exceeded while awaiting headers`, before the server-owned wait completed. See
  `BUG-20260826-bounded-wait-client-timeout`.
- Child session `ses_call_call-1a2697770f3d8ea3` completed its prompt with
  `compozy__call_return` denied by the session policy, leaving call `call-1a2697770f3d8ea3`
  running. See `BUG-20260826-call-child-tool-policy`.

## Human Verifications Needed

None identified yet.

## Decisions for a Human

None identified yet.

## Learnings

- The run keeps the seven extension hooks separate from the eleven canonical observability events; the accepted spec defines both catalogs explicitly.

## Final Status

- **Exit gate (full automated suite):** pending
- **Issues by user impact:** pending
- **Coverage:** 0 / 32 scenarios walked
- **Verdict:** pending — the matrix and fresh-lab execution must finish first.
