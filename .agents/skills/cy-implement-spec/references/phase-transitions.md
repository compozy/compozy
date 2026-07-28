# Phase transitions — detect-phase contract

`.agents/skills/cy-implement-spec/scripts/detect-phase.py` (read-only) is
the single source of truth for "what phase am I in right now?". It reads
`state.yaml` under `.compozy/tasks/<slug>/` and prints exactly one line:

```
phase=0 action=bootstrap
phase=B action=implement pending=<n> met=<m>
phase=C action=qa_report
phase=C action=qa_execution
phase=D action=peer_review round=<N>
phase=E action=done
```

The agent runs the printed action (procedures live in `SKILL.md`), repairs
every internal failure through `references/recovery-loop.md`, records the
iteration via `update-state.py`, prints the summary, then **continues** at
detect unless the outcome is a proven external blocker or Phase E. If the
session ends mid-loop, the next invocation re-runs detect-phase from
filesystem truth.

## Entry conditions

| Printed line | Entry condition |
|--------------|-----------------|
| `phase=0 action=bootstrap` | `state.yaml` does not exist. |
| `phase=B action=implement pending=<n> met=<m>` | `progress.implementation_complete=false`. The counters summarize `progress.criteria[]`; the same line re-emits every iteration until every criterion is met — there is no task queue driving Phase B, only the shrinking `pending` count. |
| `phase=C action=qa_report` | `implementation_complete=true` AND `qa.report_done=false`. Always precedes `qa_execution`. |
| `phase=C action=qa_execution` | `qa.report_done=true` AND `qa.execution_done=false`. |
| `phase=D action=peer_review round=N` | Both QA flags true AND `review.ship=false` (`N = review.rounds + 1`). Also re-emitted when `review.ship=true` but `verify.last_status != PASS` — a SHIP verdict on a failing tree is void, so review re-enters after the tree is fixed. |
| `phase=E action=done` | Both QA flags true AND `review.ship=true` AND `verify.last_status=PASS`. |

## Exit rules

- Phase 0 exits once `init-state.py` has written `state.yaml` with the
  spec's criteria; the next iteration enters B (via **continue**, not a
  restart).
- Phase B covers exactly one milestone per iteration. The phase exits when
  an iteration's `update-state.py` call carries `--implementation-complete`
  — accepted only with zero pending criteria and `--verify-pass` in the
  same call.
- Phase C produces exactly one QA artifact per iteration, `qa_report` first.
- Phase D closes one `deep-review` round per iteration via
  `--review-round-done <verdict>`; `SHIP` sets `review.ship=true`.
- Phase E prints the iteration summary plus the literal contents of
  `assets/done-signature.txt`, then stops. The codex-loop verdict prompt
  marks the goal complete from that signature.

## Failure and blocker handling (any phase)

Any command, gate, worker, or artifact failure first enters the self-healing
procedure in `references/recovery-loop.md`. Keep the current phase action open
and leave final iteration state untouched while repairing. A stale generated
artifact, failing validation, repeated repair, or third corrective touch is
not intrinsically a blocker.

Only after all three external-blocker criteria in that reference pass:

1. Record the blocker in `memory/MEMORY.md` `## Open Risks`.
2. Run `.agents/skills/cy-implement-spec/scripts/update-state.py <slug> --phase <phase> --action "external blocker: <summary>" --outcome blocked --memory-written "<paths>" --verify-fail --blocker "<text>"`
   (skip when `state.yaml` does not exist yet — bootstrap failures record in
   memory and the summary only).
3. Print the iteration summary with `outcome=blocked` and stop **without**
   the done-signature.

A later invocation re-detects the same phase until the external dependency is
available. Repairable failures never take this path.
