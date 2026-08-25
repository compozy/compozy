---
id: ET-web-marketplace-skill-install
area: ET
title: Install a skill from Marketplace and open management
persona: Bruno
journey: J-marketplace-acquisition
expected: A skill installs or updates from its stable marketplace detail, the card reflects the new installed state, and Manage opens /skills/$name for the installed skill.
entry_points: /marketplace/skill/$entryId; skill card Install or Update action
qa_status: untested
bug_ids: BUG-20260714-keyboard-focus-invisible
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38
evidence: /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-under-minute.json;/Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/web/marketplace-skill-manage.png;/Users/pedronauck/Dev/compozy/compozy/.tmp/bug-20260714-focus/focused.png;/Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/extension-charters.json;/Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-08-02-bundles-removal.md
overlaps: ET-009; ET-010; ET-api-marketplace-namespace
---

Added by marketplace Task 06. Use an entry whose display name differs from `entry_id` and install slug so detail identity, mutation identity, and Manage routing cannot collapse into one accidental field.

Historical QA note: mutation-result identity routing remains pending.

QA impact 2026-07-16: the success CTA now targets the canonical skill name returned by the install
mutation instead of catalog display metadata; reset with deliberately different names.

QA impact 2026-08-25 (skill sources): reset because the installed skill card changed. It now renders a neutral mono origin pill for skills absorbed from a non-Compozy source, and the kind page's skill query is scoped to the exact profile instead of the remembered one. Re-walk install and update from the stable detail, confirm the card still reflects the new installed state with the origin pill present only for absorbed skills, and confirm Manage still routes to the installed skill. Rides along in `CH-skill-expose-web-repair`.
