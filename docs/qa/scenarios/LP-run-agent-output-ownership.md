---
id: LP-run-agent-output-ownership
area: LP
title: Preserve typed run-agent output under daemon-owned settlement
persona: Bruno
journey: J-complete-partial-loop
expected: A run-agent task publishes only schema-valid exact structured output, preserves large results through content-addressed storage, permits agent heartbeats, and rejects agent attempts to complete or fail the daemon-owned task.
entry_points: compozy loop run; compozy loop status; /docs/loops/dsl-reference; skills/compozy/references/loops.md
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

acceptance-walk: Run a Loop whose run-agent output schema requires several fields and whose result exceeds the inline payload limit. Confirm the next node reads the exact fields, then run a result missing one required field and confirm `invalid_output`. From the bound agent session, confirm heartbeat remains available while complete and fail are rejected, and confirm the owning daemon alone records the terminal task state.
