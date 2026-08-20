---
id: LP-history-safe-repair
area: LP
title: Reuse prior Loop output safely across repair generations
persona: Bruno
journey: J-complete-partial-loop
expected: A validated history template renders schema-shaped empty values on generation 1 and sparse reattempts, renders persisted gate feedback after revision, and leaves no live Loop task after a terminal outcome.
entry_points: compozy loop validate; compozy loop run; compozy loop status; /docs/loops/reference-grammar; skills/compozy/references/loops.md
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

acceptance-walk: Validate and run a Loop whose worker prompt reads a schema-known prior node field and a gate's `blocking_issues`. Observe the initial generation, a gate-driven revision, and a sparse reattempt. Confirm each prompt materializes, the populated revision carries exact feedback, malformed dynamic data becomes an action authoring failure, and the terminal run has no queued or leased Loop tasks.
