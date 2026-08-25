---
id: RT-session-spawn-removed
area: RT
title: Reject every deleted session spawn surface
persona: Bruno
journey: J-session-spawn-removal
expected: The old CLI verb, HTTP and UDS route, native tool, schemas, and generated clients are absent while agent call remains the sole delegation surface.
entry_points: compozy spawn; POST /api/agent/spawn; compozy__session_spawn; native catalog
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-agent-call-golden-path
---

Probe each former spawn entry point and confirm a normal not-found or unknown-command response with no compatibility alias.
