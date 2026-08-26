---
id: ET-web-skill-sources-settings
area: ET
title: Manage skill sources from Settings > Skills
persona: Dora
journey: J-absorb-skills-from-other-tools
expected: Preset rows, folder paths, and counts match what the daemon measured; absent, unreadable, and unmeasured folders never render as a zero; adding and removing folders works with inline errors for a duplicate or a wrong-scope path; workspace scope states inheritance per key and returns to it; the exact workspace-profile projection is read-only; a rejected save keeps the draft and quotes the daemon
entry_points: /settings/skills; Settings > Skills sources section at user, profile, workspace, workspace-profile, and agent scope
qa_status: pass
bug_ids: BUG-20260825-custom-source-stuck-pending
fix_status: fixed
retest_status: pass
fix_commits: df739b0
evidence: /Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/browser-e2e;/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/settings-http-after.json
last_report: docs/qa/reports/2026-08-25-skill-sources.md
overlaps: ET-manage-skill-source-policy; ET-live-skill-source-reload; ET-skill-source-diagnostics-cli
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

QA plan 2026-08-25 (skill sources cycle): re-pointed from the `J-layer-profile-resources` placeholder to `J-absorb-skills-from-other-tools`, and the literal route added beside the prose scope list. Stable selectors exist for this walk (`settings-page-skills-sources` plus its `-list`, `-save`, `-message`, `-save-error`, `-defaults-only`, `-read-only`, and `-key-{sources|custom_sources}-{posture|customize|use-inherited}` children; `settings-page-skills-source-{slug}` plus `-toggle`, `-count`, `-disclosure`, `-remove`, `-truncated`, `-unreadable`; `settings-page-skills-custom-sources-{input|add|error}`; `settings-page-skills-scope-{user|workspace|agent}`), so the browser pass asserts on them rather than on copy. Two open bugs already live on this page — `BUG-20260729-skill-agent-default-selection` (agent scope opened with an unavailable identity) and `BUG-20260729-skill-policy-normalized-dirty` (a saved policy stayed marked unsaved through the same inline-save controls). Confirm whether the sources section inherits either symptom and update those files rather than filing new ids. Charter: `CH-skill-sources-settings-web`.
