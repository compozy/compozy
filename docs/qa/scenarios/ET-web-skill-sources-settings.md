---
id: ET-web-skill-sources-settings
area: ET
title: Manage skill sources from Settings > Skills
persona: Dora
journey: J-layer-profile-resources
expected: Preset rows, folder paths, and counts match what the daemon measured; absent, unreadable, and unmeasured folders never render as a zero; adding and removing folders works with inline errors for a duplicate or a wrong-scope path; workspace scope states inheritance per key and returns to it; the exact workspace-profile projection is read-only; a rejected save keeps the draft and quotes the daemon
entry_points: Settings > Skills sources section at user, profile, workspace, workspace-profile, and agent scope
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-manage-skill-source-policy, ET-live-skill-source-reload
---

Walk the section against a fixture home carrying one populated universal folder, an absent
`.claude/skills`, an unreadable custom root, and an over-cap root. Confirm each row's total and
each folder's own line come from `sources[].roots[]` — the unreadable folder shows no count at
all, the absent one reads `no folder yet`, and the over-cap one keeps its real count beside the
partial-scan sentence. Open a root's diagnostics and check the skipped links, the name clash with
its qualified form, and the verification counts against `skill sources -o json`.

Toggle a preset and confirm the count and the composer picker follow within two poll intervals.
Add a folder, then re-add the same path and a project-relative one to see the inline
`duplicate_skill_source` and `invalid_source_path` errors with the draft preserved. Switch to
workspace scope: both keys start inherited, editing one makes only that key custom, and
`Use inherited` returns it while the other stays untouched — verify the other workspace through the
API. Under a named profile, the selected workspace preserves that exact profile and exposes no
write affordance. At agent scope the source section also reads without a write affordance.

Covers UT-068–UT-071, UT-106, E2E-007–E2E-009.
