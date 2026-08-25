---
id: ET-manage-skill-source-policy
area: ET
title: Manage skill source policy across configuration scopes
persona: Dora
journey: J-absorb-skills-from-other-tools
expected: Fresh CLI and settings reads agree after source set, clear, and validation operations at user, profile, and workspace scope, while agent scope remains read-only
entry_points: compozy config get|set|unset skills.sources|skills.custom_sources --scope user|profile|workspace; [skills] sources|custom_sources in ~/.compozy/config.toml, a profile config.toml, and <ws>/.compozy/config.toml; GET /api/settings/skills?scope=user|profile|workspace|agent over HTTP or UDS; PATCH /api/settings/skills over HTTP or UDS
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-live-skill-source-reload; ET-workspace-skill-source-teammate; ET-skill-source-agent-parity
---

Walk comma-separated and JSON-array writes, explicit empty replacement, workspace inheritance via `null`, unknown and duplicate preset errors, invalid relative paths outside workspace scope, and the agent-scope policy rejection. Re-read each accepted mutation from a fresh process or request so optimistic output cannot satisfy the scenario.

QA plan 2026-08-25 (skill sources cycle): re-pointed from the `J-layer-profile-resources` placeholder to `J-absorb-skills-from-other-tools`, which this behavior actually belongs to. Entry points extended to the `config.toml` files themselves at all four layers and to `GET /api/settings/skills` across the four scopes, so the file's own read half is covered and not only the write half. Charter: `CH-skill-sources-live-apply`.
