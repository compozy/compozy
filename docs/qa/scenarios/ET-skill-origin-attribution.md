---
id: ET-skill-origin-attribution
area: ET
title: Attribute each skill to its winning source
persona: Dora
journey: J-layer-profile-resources
expected: Skill lists and detail reads agree on the source origin that supplied the winner, while Compozy-native skills use an explicit empty origin and never gain a fabricated provider label
entry_points: compozy skill list; compozy skill info; compozy skill where; GET /api/skills; GET /api/skills/{name}; compozy__skill_list; compozy__skill_search; compozy__skill_view
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-live-skill-source-reload, ET-session-command-catalog-parity
---

Load one Compozy-native skill plus skills from `agents`, `claude`, and a custom source. Confirm the CLI ORIGIN column, JSON payloads, HTTP and UDS responses, native tools, and extension Host API agree on the winning origin. Add a homonym across workspace and user roots, then verify `skill where` reports the winner, each shadow, the usable qualified-form hint, and any exposure links without inferring a custom slug from its path.
