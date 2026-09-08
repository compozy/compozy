# Phase D — peer-review rounds until SHIP

One round per iteration; detect-phase re-emits `peer_review` until the
verdict is SHIP on a verify-PASS tree. Enter this phase only after every
Phase B task or slice is complete and both QA flags are true.

1. Activate `deep-review` for the round number printed by detect-phase,
   scoped to the loop's full diff: `--base` = the ref the loop started from
   when known (default `main`), `--spec .compozy/tasks/<slug>` (contract
   conformance). Use its default reviewer runtime unless the caller selected
   another; this loop does not pin a model or require a cross-LLM lane. Later rounds ride
   deep-review's incremental state; never pass `--full` mid-loop.
2. The loop is the deciding authority over the round: remediate every confirmed finding from the round's review.md in this same iteration, nits included. A nit may be skipped only with a one-line reason recorded in the round's `memory/peer-review.md` section. Recheck only evidence invalidated by remediation, including affected QA journeys/visual rows, and satisfy the cached local commit gate. A clean review adds no new test or QA cycle. The round's verdict is the SHIP/FIX_BEFORE_SHIP/REWORK value
   in review.md/state.json.
3. Update `memory/peer-review.md` (a `## Round <N>` section per round), then
   run `python3 .agents/skills/cy-loop-tasks/scripts/update-state.py <slug> --phase D --review-round-done <SHIP|FIX_BEFORE_SHIP|REWORK> --action "peer-review round <N> (<verdict>)" --outcome completed --memory-written "memory/peer-review.md,memory/MEMORY.md" --verify-pass`.
   The call uses `--verify-pass`: a failed post-remediation gate stays inside
   the repair loop, and a SHIP verdict on a failing tree is void.
4. Run `python3 .agents/skills/cy-loop-tasks/scripts/commit-checkpoint.py <slug> --review-round <N>`
   and record the SHA or `SKIP: no changes`. Diagnose failures through
   `references/recovery-loop.md`.

Done when: the round's review.md exists with a verdict, every confirmed
finding from it is remediated or resolved with evidence, every skipped nit has its recorded reason (or the verdict was SHIP), and
`state.yaml` records the round.
