---
id: ET-skill-source-diagnostics-cli
area: ET
title: Diagnose skill source health from the CLI
persona: Dora
journey: J-diagnose-skill-sources
expected: Human and JSON CLI output report the effective source order, scope, origin, availability, truncation, skipped links, and collisions consistently for user, profile, and workspace views
entry_points: compozy skill sources; compozy skill sources -o json; compozy skill sources --json; compozy skill sources --workspace <id>; GET /api/settings/skills per-root diagnostics over HTTP or UDS; Settings > Skills per-root diagnostics at /settings/skills
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-manage-skill-source-policy; ET-live-skill-source-reload; ET-skill-source-symlink-containment; ET-web-skill-sources-settings
---

Exercise absent, unreadable, truncated, symlink-skipped, and colliding sources, including a workspace that inherits profile and user policy. Compare human output with structured JSON, verify qualified collision identities and stable precedence, then repair each source and confirm stale diagnostics disappear without a daemon restart.

QA plan 2026-08-25 (skill sources cycle): re-pointed from the `J-layer-profile-resources` placeholder to `J-diagnose-skill-sources`. Entry points now carry the canonical `-o json` form beside the existing `--json` bool, plus the settings envelope and the web disclosure that render the same per-root diagnostic schema — the parity claim only means something if all three are read in one walk. The human table's own vocabulary is the assertion target: header `SOURCE STATE WORKSPACE PATH GLOBAL PATH SKILLS NOTES`, states `always on|enabled|disabled`, notes `absent|unreadable|truncated|links skipped|collisions`, and the scope/overrides/inherits footer. Charter: `CH-skill-sources-diagnostics-truth`.
