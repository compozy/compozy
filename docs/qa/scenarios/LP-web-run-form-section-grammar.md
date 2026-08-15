---
id: LP-web-run-form-section-grammar
area: LP
title: Run form folds five sections with truthful gists and a reachable input error
persona: Dora
journey: J-05
expected: All five run-form blocks (Already running, Inputs, Participation, Environment, Limits) share the `LoopRailSection` collapse anatomy — icon, eyebrow, one-line gist, chevron. Defaults — active notice + Inputs open; Participation, Environment, Limits closed with gists `Local`/`Live · <strategy>`, `Loop default`/`worktree · <ref>`/`directory · <path>`, and the limits summary. Input names render mono with `RequiredMark`; type markers are plain faint mono, not filled chips. Limits render as hairline label/control rows with the policy select last. The action bar is one row: an idle Info sentence about dry run, the missing-input message after an invalid attempt (naming the field when only one is missing), or "Plan rendered"; Dry run (flask icon) stays clickable while invalid and paints the inline field error without creating a run; Start run stays disabled until valid; the pressed button shows a spinner while its request is in flight.
entry_points: web /loops/:name/run
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits: eb43da0f
evidence:
last_report:
overlaps: LP-loop-input-defaults; LP-loop-environment-resolution
---

Added by the loops visual-contract parity pass (2026-08-14). The dry-run gating change is the behavioral core: pointer users can now reach the required-input error state. Walk needs a loop with required + optional inputs and an active concurrent run; deferred to the next seeded QA cycle — `loop-run-form.test.tsx` (invalid dry run paints the error and fires no request; blank submit creates nothing) is green at 9a694ff2.
