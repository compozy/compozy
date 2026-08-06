# QA Run Report — 2026-08-06 — Loop Agent Ownership

- **Scope:** Fresh dev-cycle enablement, hosted-tool availability, exclusive Loop-worker ownership, durable cancellation delivery, resumed-session ledger continuity, and the adjacent task-catalog polling contract.
- **Cadence tier:** broad playbook plus targeted acceptance walks
- **Build:** `a00a9df50ed0` plus the current working tree
- **Environment:** isolated lab `compozy-loop-agent-ownership-r2-20260806-040706-936266`
- **Behavioral run:** 2026-08-06T04:07:41Z–2026-08-06T04:53:56Z

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Lea | New User | laptop / wifi-fast / en-US | CH-001 |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-loop-worker-exclusive-dispatch, CH-003, CH-038 |
| Ada | Autonomous Agent | desktop / wifi-fast / en-US | CH-managed-session-skill-loading, CH-mcp-protocol-interop |
| Théo | Power User | desktop / wifi-fast / en-US | CH-stopped-session-prompt-continuity |
| Priya Joshi | Head of Growth | desktop / wifi-fast / en-US | consumer-saas-growth |

## Session Matrix & Results

| # | Journey / Scenario | Persona | Tour | Status | Evidence |
|---:|---|---|---|---|---|
| 1 | J-01 / ET-052 | Lea | Feature Tour | Pass | `extension-disabled-after-restart-status.json`, `extension-enabled-final-tool.json` |
| 2 | J-complete-task-tree / TA-loop-worker-exclusive-dispatch | Bruno | Feature Tour | Pass | `loop-owner-sessions.json`, `loop-cancel-events.sse` |
| 3 | J-complete-task-tree / TA-task-role-session-activation | Bruno | Feature Tour | Pass | `task-list.json`, `growth-task-runs-http.json` |
| 4 | J-load-skill-in-managed-session / ET-managed-session-skill-loading | Ada | Feature Tour | Pass | `operator-kickoff.jsonl`, `growth-pm-hosted-tools-session.png` |
| 5 | J-validate-compozy-hard-cut / ET-compozy-native-tool-invocation | Ada | Feature Tour | Pass | `operator-kickoff.jsonl`, `extension-enabled-final-tool.json` |
| 6 | J-recover-loop-node-failure / LP-016 | Bruno | Interrupt Tour | Pass | `loop-cancel-draining.json`, `loop-cancel-events.sse` |
| 7 | J-recover-loop-node-failure / LP-cancel-vs-kill | Bruno | Interrupt Tour | Pass | current cancel walk plus the unchanged kill evidence from 2026-08-03 |
| 8 | J-13 / RT-018 | Théo | Interrupt Tour | Pass | `session-resume-prompt.jsonl`, `session-resume-ledger-replaced.txt` |
| 9 | J-24 / TA-002 | Bruno | Feature Tour | Pass | `tasks-live.png`, `tasks-reloaded.png` |
| 10 | consumer-saas-growth | Priya Joshi | Task Tour | Blocked | one task could not run its database check because local `psql` is absent; some channel handoffs also reported an unavailable backend |

All nine changed or adjacent product scenarios reached `qa_status: pass`. The broad playbook row is
reported separately because its environment and collaboration limits do not contradict the
targeted product evidence.

## Session Debriefs

### Fresh bundled extension lifecycle

A fresh isolated home installed `dev-cycle` as `enabled: true`, `state: active`, and healthy. Its
two Loops, three bundled agents, and three extension-host tools were available immediately. After
`compozy extension disable dev-cycle`, the Loop catalog became empty, the bundled agents left the
resolved agent catalog, and `ext__dev_cycle__import_tasks` returned `tool_not_found`. The disabled
choice survived a full daemon restart. Re-enabling the extension restored the same Loops, agents,
and callable tool descriptor.

### Hosted tools and ordinary task workers

The real Growth PM Codex session `sess-e637063ebeaee710` called hosted
`compozy__tool_info`, `compozy__skill_view`, and task tools from its normal managed session. Seven
workspace agents then executed ordinary task-role runs. Ten seeded runs completed; the analytics
run that required `psql` failed after writing its SQL and test artifacts because that executable
is not installed in the lab.

### Exclusive Loop worker and durable cancel

Loop run `looprun-922064c3e2202a8b` created exactly one reviewer worker session,
`sess_c804babe70212c41a4e4c709b7521364`, attached to run
`run.loop.looprun-922064c3e2202a8b.g1.node.review.0`. No coordinator or ordinary task-role session
claimed the Loop worker. Public cancel first exposed `cancel_state: draining`; 63 seconds later the
run reached canceled with cause `operator_cancel`, and SSE recorded the node cancellation and one
terminal transition.

### Stopped-session resume and ledger replacement

The Growth PM session was stopped, then received a normal follow-up prompt. It resumed under the
same Compozy session ID and the same ACP session ID, recalled the earlier launch decision, and
kept the launch on hold. Its materialized ledger was absent while resumed and was recreated after
the second stop with both the original kickoff and the follow-up turn. No duplicate-ledger error
occurred.

### Task catalog refresh contract

CLI, HTTP, and Web showed the same 11-task catalog. Browser performance evidence recorded one
main task-catalog request over more than 35 seconds; no hidden interval refetch occurred. A manual
reload fetched the new task state. The approval inbox continued its independent five-second poll,
which is outside the task-catalog contract.

## Disruption Probes

- **Silent event drop:** the shared event-volume file changed to `first_save: 0`; the Data Scientist
  and Growth PM retained the launch hold. The agent did not explicitly quote the refreshed zero in
  its handoff, so the report does not claim stronger autonomous propagation.
- **Variant assignment skew:** a public 70/30 warning woke the Experiment Engineer. A fresh
  100,000-sample check produced 50.065% / 49.935%, all 11 tests passed, and the agent held
  production allocation pending upstream evidence.
- **Lifecycle send misfire:** a real task/run completed with the send paused, an incident decision,
  and explicit release gates.

## Limitations and Runtime Errors

- The broad playbook is `BLOCKED`, not failed product acceptance: local `psql` is unavailable, so
  one generated SQL artifact could not be exercised against PostgreSQL.
- Several agent channel sends reported `backend_unhealthy` even though later operator Network
  sends succeeded. The resulting playbook handoffs are incomplete and are not used as proof for
  issues 321 or 322.
- `observe-runtime.py` reported `stall_detected: true` because the journey log had task-start rows
  without task-completion rows. Public runtime evidence showed 10 completed runs and one failed
  run. `qa/issues/BUG-observer-stale-task-completions.md` records this QA-tooling limitation.
- The first public `daemon stop` exceeded the CLI timeout while one task-role session drained, then
  completed without a forced signal. The restarted daemon used PID `56932` and remained registered
  for mandatory teardown.

## Human Verifications Needed

None for the changed product behavior. Re-running the broad analytics task on a host with
PostgreSQL tooling would improve the playbook verdict but is not needed to validate issues 321 and
322.

## Final Status

- **Changed product scenarios:** PASS — 9/9 terminal passes.
- **Broad business playbook:** BLOCKED by the missing `psql` executable and incomplete channel
  handoffs; all required deliverable files were still produced.
- **Final make verify evidence:** `/Users/pedronauck/dev/qa-labs/compozy-loop-agent-ownership-r2-20260806-040706-936266-lab/qa-artifacts/qa/evidence/final-make-verify.log`
- **Teardown evidence:** `/Users/pedronauck/dev/qa-labs/compozy-loop-agent-ownership-r2-20260806-040706-936266-lab/qa-artifacts/qa/teardown.json` (`clean: true`).
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0.
- **Verdict:** PASS for the #321/#322 acceptance contract; the unrelated broad playbook remains
  BLOCKED for the environment limits above.
