---
id: RT-calls-config-effects
area: RT
title: Apply calls configuration through agent-manageable surfaces
persona: Bruno
journey: J-calls-config
expected: Config get/set exposes every calls key and new calls honor depth, batch, child, idle, result budget, message, and overflow limits without changing in-flight snapshots.
entry_points: compozy config get/set calls.*; config.toml; calls runtime
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-agent-call-batch; TA-task-result-contract
---

Change one calls limit at a time, verify the public config readback, and exercise its observable runtime boundary.
