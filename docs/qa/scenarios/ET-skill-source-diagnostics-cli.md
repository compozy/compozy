---
id: ET-skill-source-diagnostics-cli
area: ET
title: Diagnose skill source health from the CLI
persona: Dora
journey: J-layer-profile-resources
expected: Human and JSON CLI output report the effective source order, scope, origin, availability, truncation, skipped links, and collisions consistently for user, profile, and workspace views
entry_points: compozy skill sources; compozy skill sources --json; compozy skill sources --workspace <id>
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-manage-skill-source-policy, ET-live-skill-source-reload
---

Exercise absent, unreadable, truncated, symlink-skipped, and colliding sources, including a workspace that inherits profile and user policy. Compare human output with structured JSON, verify qualified collision identities and stable precedence, then repair each source and confirm stale diagnostics disappear without a daemon restart.
