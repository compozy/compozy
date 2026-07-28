# L-028 — A correction is not a license for the opposite extreme

**Class:** Decision process / Spec authoring
**Date discovered:** 2026-07-07
**Evidence sources:** Decision sequence of the loops × task-domain architecture session (2026-07-07).

## Context

In the loops × task-domain session, a proposal to build attention-unification machinery (routing loop human gates into the task inbox) was corrected by the operator as over-engineering: watching events is composition, and attention routing is not the loop engine's responsibility. The immediate next position swung to the opposite extreme — "then the existing core human-gate machinery is the excess; shrink it to contract boundaries and compose human decisions via approval tasks or channel harvest."

That swing produced a second error on the same axis **and** a direct contradiction with a premise already accepted earlier in the same discussion: it had just been established that not every loop composes with tasks (run-agent chains are first-class), yet the new position proposed composing human decisions *via tasks*. The operator had to correct the same axis twice. The premise-derived answer was asymmetric and kept parts of both positions: keep both domain-native attention surfaces, and build no unification machinery.

## Root cause

A correction was treated as a negation of the prior position instead of a trigger to re-derive each affected conclusion independently from the stated premises. Positions on a design axis are rarely binary; negating the last statement optimizes for appearing responsive, not for being right, and it skips the contradiction check against premises already settled in the same discussion.

## Rule

> After a correction, re-derive from premises — do not negate the last position. Before replying, check the new position for contradictions against every premise already accepted in the discussion; if the new position contradicts one, the swing is wrong, not the premise.

## Operationalization

- Keep an explicit numbered premise list (P1..Pn) during design discussions; the final spec brief records them as settled premises with rationale.
- When corrected, walk each affected conclusion against every premise and name which premise drives the revision — "P4 (delegation is opt-in) rules out composing human decisions via tasks" would have caught the swing immediately.
- Treat a second correction on the same axis as a signal that the reasoning mode (pendulum) is the bug, not the specific position.

## Anti-pattern

- "You're right — so the opposite" replies.
- Deleting or deprecating an existing primitive as penance for having over-engineered around it.
- Accepting a correction without re-checking previously settled premises for contradiction with the new position.

## Source

- Settled premises P4–P6 and non-goals 3–4 recorded in the session's spec brief (`ai-docs/loops-task-integration-spec-brief.md`, local artifact; premises restated in the lesson body above so this file stands alone).
- The machinery that was wrongly proposed for removal and correctly kept: `internal/loop/service_types.go:45-46` (`needs-approval` status), `internal/store/globaldb/migrate_loop_run_pinning.go:20-37` (`loop_gate_decisions`), gate criteria vocabulary in `internal/loop/gate/`.
