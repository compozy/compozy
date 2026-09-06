# Phase E — done

For `action=done`, check that the recorded PR heads/checks and local evidence
still cover the current delivery. If valid, continue at step 5 without another
push, gate run, or CI-pending state update. Otherwise refresh the affected evidence
through the steps below.

1. Assess final delivery through `cy-final-verify`, reusing the diff review,
   gate, QA, and visual evidence already collected. Use the cached `make gate`
   for commit/push; rerun only invalidated checks. Confirm
   `state.verify.last_status=PASS` and that all delivery obligations are covered.
2. Push the current head and create or update its draft PR. In stacked mode,
   resubmit the stack and include every open layer; otherwise use the current
   branch PR. Record pending evidence with `update-state.py <slug> --phase E
   --action "await exact-head PR CI" --outcome partial --ci-pending --pr-url
   <url> --head-sha <sha>`; repeat the paired URL/SHA flags in stack order.
3. Watch every PR's checks to terminal state (`gh pr checks --watch
   --interval 20`). A pending or red check keeps Phase E open; diagnose,
   repair, rerun the affected local gate, checkpoint, push, and watch the new
   head. Only green required checks at the recorded head pass. Then record
   fresh terminal evidence with `update-state.py <slug> --phase E --action
   "exact-head PR CI passed" --outcome completed --ci-pass --pr-url <url>
   --head-sha <sha> --ci-check <name>` (repeat paired PR and check flags as
   needed). The script rejects stale heads and empty check sets.
4. Confirm the Phase E completion conditions in `references/checklist.md`.
   This is an evidence audit, not another execution of the checks.
5. Print the iteration summary block from
   `assets/iteration-summary.template.md` with `phase_out=E` and checkpoint
   field `n/a (phase != B/D)`, followed by every entry from
   `memory/MEMORY.md` `## Open Questions` — the decisions defaulted
   mid-loop surface to the user here, in one batch.
6. Print the literal contents of `assets/done-signature.txt` on its own line
   — the codex-loop goal-check confirmation scans for it.
7. Stop — Phase E is the only successful terminal.

Done when: the local gate and exact-head PR CI are green, the Phase E
checklist passes, and the done-signature is the final output line.
