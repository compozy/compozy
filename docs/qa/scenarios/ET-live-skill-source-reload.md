---
id: ET-live-skill-source-reload
area: ET
title: Reload configured skill sources without restarting the daemon
persona: Dora
journey: J-layer-profile-resources
expected: Source enablement, disablement, replacement, and scan-health changes become visible through skill resources, settings diagnostics, and agent envelopes within two watcher intervals without restarting the daemon
entry_points: PATCH /api/settings/skills over HTTP or UDS; compozy config set skills.sources; resources/list; agent session envelope
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-manage-skill-source-policy
---

Start with preset and custom sources containing distinct skills, then disable, enable, and replace them while the daemon remains running. Confirm removed skills disappear, newly enabled skills appear, workspace inheritance remains isolated, and truncation or skipped-link diagnostics set and clear after the source is repaired. Read each surface from a fresh request and allow no more than two configured watcher intervals for convergence.
