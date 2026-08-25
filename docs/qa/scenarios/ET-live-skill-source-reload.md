---
id: ET-live-skill-source-reload
area: ET
title: Reload configured skill sources without restarting the daemon
persona: Dora
journey: J-absorb-skills-from-other-tools
expected: Source enablement, disablement, replacement, and scan-health changes become visible through skill resources, settings diagnostics, and agent envelopes within two watcher intervals without restarting the daemon
entry_points: PATCH /api/settings/skills over HTTP or UDS; compozy config set skills.sources; compozy skill sources; Settings > Skills sources section at /settings/skills; resources/list; agent session envelope
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

QA plan 2026-08-25 (skill sources cycle): re-pointed from the `J-layer-profile-resources` placeholder to `J-absorb-skills-from-other-tools`. Entry points extended with `compozy skill sources` and the Settings route, because the live-apply promise is only settled when the operator-facing meter converges too, not just the resource and envelope reads. Charter: `CH-skill-sources-live-apply`.
