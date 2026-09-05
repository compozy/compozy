---
id: ET-terminal-lease-fencing
area: ET
title: Keep exactly one writer on a terminal through takeover, disconnect, and recovery
persona: Marina
journey: J-supervise-agent-terminal
expected: A terminal has exactly one writer at every instant; a human takeover lands atomically and the agent is told rather than starved; a non-controller write is refused instead of queued for a later prompt; and a stale agent from before a run end or runtime restart changes nothing at all.
entry_points: Terminal app header take-control and release; compozy terminal attach --control and --force; compozy__terminal_claim; compozy__terminal_write; compozy__terminal_yield; terminal lease events
qa_status: skipped
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-review-r2-20260902-020216-937662-lab/qa-artifacts/qa/logs/stream-lease-redaction-session.md; docs/qa/reports/2026-09-01-integrated-terminal-review-r2.md
last_report: docs/qa/reports/2026-09-01-integrated-terminal-review-r2.md
overlaps: ET-terminal-agent-handoff-input; ET-terminal-hook-events
---

Retired 2026-09-04: the exclusive writer lease, takeover, yield, claim, and generation-fencing contract
were removed before release. `ET-terminal-shared-control` owns the replacement behavior. Historical
evidence remains linked here because this file is durable QA memory.

qa-impact: 2026-09-01 deep-review round 2 changed lease release, reconnect, and stale-controller
cleanup paths. Reset for a focused contested-control and recovery re-walk.

2026-09-02 re-walk: passed. Human takeover rejected the bound agent's write with typed
`lease_revoked` and no terminal or journal side effect; closing the first controlling view preserved
control, the final close released it after grace, and a foreign agent run remained fenced. Confirmed,
cancelled, and forced human takeover paths retained exactly one writer, while watcher input was ignored.

Planned by integrated-terminal task 09 for the control-lease guarantees. `ET-terminal-agent-handoff-input`
walks the ordinary watch-claim-yield path; this file owns the contested and recovery paths that the
lease model exists to survive. Task 10 owns the walk, evidence, and verdict.

Walk:

1. While an agent is typing, take control from the browser and confirm the switch is instant, that no
   half-applied agent input reaches the program, and that the agent's next call reports the new
   controller instead of hanging.
2. Have the agent write while it does not hold control and confirm the write is refused outright — then
   let it take control legitimately and confirm no earlier refused write arrives at the new prompt.
3. Claim control from a second human client and confirm the confirmation is asked for, that declining
   leaves the controller unchanged, and that the forced form skips only that confirmation.
4. Confirm an agent can never take control away from a human the way a human takes it from an agent.
5. End the agent's run while it holds control and confirm control returns to the human exactly once,
   with one recorded transition and no duplicate.
6. Restart the runtime, then let the pre-restart agent act on the same terminal; confirm the stale
   attempt is refused with a fencing answer and leaves no output, journal row, or state change behind,
   and that the recovered run must re-list before it can work again.
7. Open the same terminal in two tabs as the same controller, close one, and confirm control is not
   released; close the second and confirm control is released only after the grace period, then confirm
   a bound agent can claim it afterwards.
