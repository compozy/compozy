---
id: TA-task-result-contract
area: TA
title: Complete a task against its run-start result contract
persona: Bruno
journey: J-task-result-contract
expected: Task authoring accepts expect and budget fields, reads echo the digest and effective budget, and completion validates the immutable run-start snapshot with one resubmission.
entry_points: compozy task create/update/complete; task API; native task tools
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Start a contracted run, update the task contract mid-run, submit one invalid result, then resubmit against the original snapshot.
