---
id: ET-skill-origin-attribution
area: ET
title: Attribute each skill to its winning source
persona: Dora
journey: J-absorb-skills-from-other-tools
expected: Skill lists, source filters, and detail reads agree on the source origin that supplied the winner, while Compozy-native skills use an explicit empty origin and never gain a fabricated provider label
entry_points: compozy skill list; compozy skill list --source bundled|marketplace|user|profile|additional|workspace|workspace_profile|agent-local; compozy skill info; compozy skill where; GET /api/skills; GET /api/skills/{name}; compozy__skill_list; compozy__skill_search; compozy__skill_view; extension Host API skills/list; /docs/skills/sources
qa_status: pass
bug_ids: BUG-20260825-skill-detail-rejects-workspace-id
fix_status: fixed
retest_status: pass
fix_commits: 740d868cf
evidence: /Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/origin-native-summary.json;/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/list-workspace.json;/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/info-qa-source.json
last_report: docs/qa/reports/2026-08-25-skill-sources.md
overlaps: ET-live-skill-source-reload; ET-session-command-catalog-parity; ET-skill-source-agent-parity; ET-skill-source-symlink-containment
---

Load one Compozy-native skill plus skills from `agents`, `claude`, profile, workspace-profile, and a custom source. Confirm the CLI ORIGIN column, JSON payloads, HTTP and UDS responses, native tools, and extension Host API agree on the winning origin. Filter `skill list` by every emitted public tier, including `profile` and `workspace_profile`, and confirm each result belongs to that tier. Add a homonym across workspace and user roots, then verify `skill where` reports the winner, each shadow, the usable qualified-form hint, and any exposure links without inferring a custom slug from its path.

QA plan 2026-08-25 (skill sources cycle): re-pointed from the `J-layer-profile-resources` placeholder to `J-absorb-skills-from-other-tools`. Entry points now name all eight accepted `--source` tier values verbatim, the extension Host API `skills/list` method (its `SkillSummary` gained `origin` in this diff), and the published `/docs/skills/sources` page — a doc that describes origin wrongly is a user-visible failure on this same promise. Charter: `CH-skill-sources-live-apply`.
