# L-039 — Self-asserted quality is no gate; a green test can bless the bug

**Class:** Process / Testing / Verification
**Date discovered:** 2026-09-01
**Evidence sources:** agent-comms post-mortem 2026-09-01; `.compozy/tasks/agent-comms/reviews/summary.md`, `state.yaml`, and the E2E fixtures on the unmerged `agent-comms` branch — remote only, draft PR [compozy/compozy#497](https://github.com/compozy/compozy/pull/497). Spec set: [`compozy-specs/_archived/2026-08-19-agent-comms/`](https://github.com/compozy/compozy-specs/tree/main/_archived/2026-08-19-agent-comms).

## Context

During agent-comms' single implementation day, the executing loop violated 22 of the 27 skill rules its own task files cited (the post-rejection review's compliance table: 22 `falhou`, 4 `parcial`, 1 `passou`) — including rules that would have prevented the worst defects. The loop then issued its own SHIP verdict two minutes after QA closed (`review.rounds: 1`), and ~350 of the ~700 total findings arrived only after the owner rejected the result and commissioned independent review slices.

The suite also contained an **inverted golden**: a deterministic E2E fixture (`silent`) ratified the buggy behavior (prose falling through to `completed-without-result`), so fixing the production defect turned CI red. Project rules covered the forward direction — "a test exposes a bug → fix the production code" — but not the inverse: a green test that certifies spec-contradicting behavior.

L-031 had already isolated the pattern for one invariant: "Every other design-system invariant had a deterministic gate; component reuse had only prose." This incident is its generalization — under a 17.5-hour velocity push, **every** prose-only invariant failed, and every deterministically gated one held.

## Root cause

Quality claims (skill compliance, SHIP, VC parity, QA verdicts) were self-asserted by the same run they judged, with no deterministic gate or independent check behind them. Prose rules do not survive velocity pressure; only gates do. And a golden fixture written from observed behavior rather than from the contract encodes the bug as the standard.

## Rule

> A quality claim counts only when a gate or an independent verifier stands behind it: SHIP verdicts come from a reviewer lane that did not implement the work; skill-cited invariants that matter get deterministic checks, not prose. A test or fixture that asserts behavior contradicting the spec contract is a production bug — fix the fixture and the code together; red-after-fix is the proof the golden was inverted, never a reason to keep the bug.

## Operationalization

- Fixtures and goldens derive from the spec contract (`_tests.md`, `_dx.md` examples), never from capturing current output; a contract change re-derives them.
- When writing a rule an agent must obey under pressure, ask "what turns red when this is violated?" — no answer means the rule needs a gate or should not be relied on (L-031 precedent).
- Review independence: the implementing lane never solely reviews its own work (`deep-review --subagent` cross-LLM lane is the floor).

## Anti-pattern

- SHIP minutes after the last self-run check, with no independent eyes on the diff.
- "The E2E passes" as a defense when the E2E asserts the defect.
- Weakening the fixture (or the spec) to keep CI green after a production fix — the inverse of the CLAUDE.md test rule, same dishonesty.

## Source

- On the `agent-comms` branch (remote), under `.compozy/tasks/agent-comms/`: `reviews/summary.md` skills-compliance table (22/27 `falhou`) and slice 12 P0 ("Consertar 09 F-001/F-007 deixa a suíte vermelha"); `state.yaml` (`review.rounds: 1`, SHIP timestamp 2 minutes after QA).
- Generalizes: `L-031-primitive-reuse-is-a-gate-not-prose.md`.
