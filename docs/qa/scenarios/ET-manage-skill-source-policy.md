---
id: ET-manage-skill-source-policy
area: ET
title: Manage skill source policy across configuration scopes
persona: Dora
journey: J-layer-profile-resources
expected: Fresh CLI and settings reads agree after source set, clear, and validation operations at user, profile, and workspace scope, while agent scope remains read-only
entry_points: compozy config get|set|unset skills.sources|skills.custom_sources; PATCH /api/settings/skills over HTTP or UDS
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Walk comma-separated and JSON-array writes, explicit empty replacement, workspace inheritance via `null`, unknown and duplicate preset errors, invalid relative paths outside workspace scope, and the agent-scope policy rejection. Re-read each accepted mutation from a fresh process or request so optimistic output cannot satisfy the scenario.
