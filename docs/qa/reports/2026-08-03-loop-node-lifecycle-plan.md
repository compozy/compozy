# QA Plan — 2026-08-03 — Loop node lifecycle

- **Scope:** Final real-user cycle for the node lifecycle and failure contract implemented by Tasks 02–11, across authoring, operation, approval, agent-native management, durable restart behavior, and Web truthfulness.
- **Cadence tier:** full — the change crosses runtime, persistence, effects, CLI, HTTP, UDS, native tools, SSE, and three Web journeys.
- **Status:** planning only. No runtime was launched, no persona session ran, and no verdict is recorded here.
- **Execution report:** Task 13 creates `docs/qa/reports/<YYYY-MM-DD>-loop-node-lifecycle.md` from the local report template before the first session and updates it after every session and fix.

## Flag reconciliation

Every QA-impact flag from Tasks 02–11 has exactly one content-addressed scenario owner. Existing
files were updated in place and reset to `untested`; no sibling scenario or counter id was minted.

| Task | QA-impact flag | Canonical scenario | Planned charter |
|---|---|---|---|
| 02 | retry-heals | `LP-transient-blip-heals` | `CH-author-loop-failure-contract` |
| 02 | route-fallback | `LP-error-route-fallback` | `CH-author-loop-failure-contract` |
| 02 | escalation | `LP-unannotated-escalation` | `CH-author-loop-failure-contract` |
| 03 | notification: on-error context | `LP-on-error-notification-with-context` | `CH-author-loop-failure-contract` |
| 03 | notification: committed terminal | `LP-terminal-outcome-notification` | `CH-author-loop-failure-contract` |
| 04 | sick-target | `LP-sick-target-degrades-one-lane` | `CH-operator-loop-recovery` |
| 04 | quarantine-requeue | `LP-quarantine-diagnose-requeue` | `CH-operator-loop-recovery` |
| 05 | death-resume | `LP-crash-death-resume` | `CH-operator-loop-recovery` |
| 05 | cancel-vs-kill | `LP-cancel-vs-kill` | `CH-operator-loop-recovery` |
| 05 | days-long-node | `LP-days-long-node-no-clock` | `CH-operator-loop-recovery` |
| 06 | pause-repair | `LP-live-pause-repair-resume` | `CH-operator-loop-recovery` |
| 06 | approval-link | `LP-approval-link-journey` | `CH-approver-loop-wait` |
| 06 | durable-wait-restart | `LP-durable-wait-restart` | `CH-agent-loop-lifecycle-parity` |
| 06 | inventory-escalation | `LP-waiting-inventory-escalation` | `CH-agent-loop-lifecycle-parity` |
| 06 | duplicate-suppressed | `LP-duplicate-event-suppressed` | `CH-agent-loop-lifecycle-parity` |
| 07 | agent-native-tools | `LP-agent-operates-lifecycle-via-native-tools` | `CH-agent-loop-lifecycle-parity` |
| 08 | operator-lifecycle-ui | `LP-operator-lifecycle-ui` | `CH-operator-loop-recovery` |
| 09 | editor-authoring-walk | `LP-editor-authoring-walk` | `CH-author-loop-failure-contract` |
| 10 | catalog-runform-walk | `LP-catalog-runform-walk` | `CH-author-loop-failure-contract` |

Four grandfathered scenarios also touch the removed `loop stop` contract and are reset in place:
`LP-016` owns the historical run-level semantic regression, `TA-070` owns HTTP/UDS route parity,
`TA-076` owns native-tool catalog parity, and `TA-084` owns the Web Stop→Cancel hard cut. Their
overlaps point to content-addressed owners `LP-cancel-vs-kill` and
`LP-agent-operates-lifecycle-via-native-tools`; none is a second owner for a Task 02–11 flag.

## Journeys in scope

- [`J-recover-loop-node-failure`](../journeys/J-recover-loop-node-failure.md) — Lea authors and starts the contract; Bruno observes failure, preserves healthy work, repairs the affected lane, and reaches a truthful finish. It includes restart and leave-while-parked abandonment paths.
- [`J-03`](../journeys/J-03-observe-and-approve.md) — Marina follows the exact approval link, survives a dropped request, and confirms one durable decision through Web and structured reads.
- [`J-07`](../journeys/J-07-agent-operated-run.md) — Ada manages lifecycle actions and durable waits through native tools, then proves CLI/HTTP/UDS parity, restart durability, deduplication, and workspace isolation.

## Session matrix, ordered by risk

| Order | Charter | Persona | Journey | Scenarios | Tour | Planned proof |
|---:|---|---|---|---|---|---|
| 1 | [`CH-operator-loop-recovery`](../charters/CH-operator-loop-recovery.md) | Bruno — operator | `J-recover-loop-node-failure` | sick target; quarantine/requeue; death resume; cancel/kill; legacy Stop→Cancel; days-long node; pause/repair; lifecycle UI | Interrupt Tour | Live Web state and allowed actions match CLI/HTTP/UDS after interruption, restart, race, and refresh. |
| 2 | [`CH-agent-loop-lifecycle-parity`](../charters/CH-agent-loop-lifecycle-parity.md) | Ada — managing agent | `J-07` | native lifecycle tools; HTTP/UDS route parity; native stop-tool absence; durable wait restart; wait inventory/escalation; duplicate suppression | Feature Tour | Native outputs field-diff cleanly against CLI/HTTP/UDS, remain workspace-scoped, and survive restart. |
| 3 | [`CH-author-loop-failure-contract`](../charters/CH-author-loop-failure-contract.md) | Lea — Loop author | `J-recover-loop-node-failure` | editor; catalog/run form; retry; route; repair; on-error effect; terminal effect | Feature Tour | The published daemon definition and real recovery outcome agree across Web, CLI, HTTP, and SSE. |
| 4 | [`CH-approver-loop-wait`](../charters/CH-approver-loop-wait.md) | Marina — approver | `J-03` | exact durable approval link | Network Tour | A throttled or duplicated decision has one winner and the same fresh state on Web, CLI, HTTP, and UDS. |

The four charters collectively cover CLI, Web, HTTP API, UDS, native tools, and SSE. Each charter
names one tour, one persona, one journey, a time-box, checkpoint/failure evidence, and its independent
read path. The author, operator, and approver UI legs use `browser-use:browser` as the primary
Playwright-backed driver and `agent-browser` only as a recorded fallback. Ada's session stays
headless by persona contract; the three UI charters own Web parity.

## Isolated Task 13 lab

Task 13 must create a fresh lab; planning does not allocate or launch one. Start with:

```bash
python3 .agents/skills/eng/eng-qa-bootstrap/scripts/bootstrap-qa-env.py \
  --scenario "loop-node-lifecycle" \
  --repo-root .
```

Record the emitted `BOOTSTRAP_MANIFEST` and source only its values. The manifest must be healthy and
must provide unique, non-default `COMPOZY_HOME`, `COMPOZY_HTTP_PORT`, `COMPOZY_UDS_PATH`, and
`TMUX_BRIDGE_SOCKET`, plus `COMPOZY_WEB_API_PROXY_TARGET`, `PROVIDER_HOME`, `PROVIDER_CODEX_HOME`,
`BROWSER_MODE`, `QA_OUTPUT_PATH`, `AUDIT_COMMAND`, and `TEARDOWN_COMMAND`. The required handoff files
are `bootstrap-manifest.json`, `bootstrap.env`, `scenario-contract.json`,
`behavioral-scenario-charter.yaml`, `journey-log.jsonl`, and `provider-attempt.json`.

Execution rules:

1. Verify the final automated precondition with `make test-e2e-runtime`; this is not a substitute for the persona sessions.
2. Export the manifest-derived `COMPOZY_WEB_API_PROXY_TARGET` before starting Web, preserve operator home only for `native_cli + home_policy=operator`, and serialize config writes within the lab.
3. Register every daemon, Web server, watcher, browser session outside its driver's lifecycle, and tmux process immediately under `<QA_OUTPUT_PATH>/qa/pids/`.
4. Append each meaningful browser, CLI, HTTP, UDS, native-tool, runtime, and provider action to `journey-log.jsonl`; checkpoint and failure screenshots go under the lab output and are linked from the execution report.
5. Before a behavior-first verdict, run `python3 "$AUDIT_COMMAND" --qa-output-path "$QA_OUTPUT_PATH" --strict` and record its exit result.
6. On pass, fail, blocked, or abort, run the exact `eval "$TEARDOWN_COMMAND"`. Task 13 cannot close until `<QA_OUTPUT_PATH>/qa/teardown.json` reports `"clean": true`; files may remain for forensics, processes may not.

Reuse is forbidden for a new pass. It is allowed only when Task 13 continues the same active session
with the exact manifest path and records the machine-readable continuation block from the bootstrap
contract.

## Cross-surface comparison protocol

For every state-changing step, record the exact object id, actor, workspace, command, HTTP request,
and response. The CLI leg uses structured JSON. The HTTP leg uses the matching public route. A pass
requires field-level agreement on state, terminal cause, attempt, next-attempt time, wait identity,
provenance, allowed transitions, effect result, and workspace id where each field applies. The Web
leg reloads or deep-links the exact object after that comparison; an optimistic screen is not proof.

`make test-e2e-runtime` proves the automated runtime harness is green on the final build. The four
sessions prove real public-interface behavior. Neither is allowed to stand in for the other.

## Taxonomy decisions

- **Journeys:** all three value flows reach their true end state; author/operator and approval flows include abandonment and resume paths.
- **Functional:** each scenario has an executable acceptance walk and an independent public read path.
- **Experiential:** browser sessions record usability, accessibility, perceived-performance, compatibility, recoverability, and production-parity observations; the editor's mobile layout is deliberately out of scope because the product declares it desktop-only.
- **Edge, error, empty:** retry, route, unhandled repair, breaker, quarantine, managed death, pause, cancel/kill, network drop, stale approval, restart, duplicate event, absent status filter, and invalid publish are covered.
- **Cross-cutting:** workspace A/B isolation, restart continuity, CLI/HTTP/UDS/native parity, Web refresh/deep-link truth, 4G approval, keyboard interaction, and one adjacent catalog/detail canary are covered.

## Task 13 preflight and close conditions

- Materialize the tracker from scenario frontmatter; every file parses and all 23 in-scope/reset rows start `untested`.
- Create the execution report before the first session with all matrix rows `Pending`; update it and scenario verdict fields immediately after each session.
- Every charter reaches a terminal session status and has a debrief. Any `Fail` links a deduplicated bug registry entry; any governed fix gets a regression proof and same-persona re-walk.
- Every scenario added or reset by Tasks 02–11 finishes `pass`, `blocked-verify`, `blocked-decision`, or an honestly reasoned `skipped`; zero rows remain `untested` or `fail` at workstream completion.
- Run the final workstream gate only after the last mutation, and cite both fresh gate evidence and clean teardown evidence in the execution report.

## Compozy Impact Audit

- **Native tools:** no descriptor, schema, digest, risk flag, or capability gate changes in this planning task; Task 13 checks the eight existing lifecycle IDs, absence of `compozy__loop_stop`, deterministic loser results, and HTTP/UDS parity.
- **Extensibility and hooks:** no extension, hook, skill, bundle, registry, bridge SDK, MCP sidecar, or config lifecycle change in this planning task; the sessions exercise existing lifecycle effects and watch-source admission through their public outputs.
- **Workspace data isolation:** no datum changes in this planning task; node inventory, wait identity, dedup claims, events, native tools, CLI, HTTP, UDS, SSE, and Web caches are checked with workspace A/B negative controls.
- **Official Compozy skill:** no skill behavior change in this planning task; `skills/compozy/` was updated by Task 11 and Task 13 checks its documented lifecycle paths against the final runtime.
