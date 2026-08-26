---
id: ET-web-marketplace-skill-install
area: ET
title: Install a skill from Marketplace and open management
persona: Bruno
journey: J-marketplace-acquisition
expected: A skill installs or updates from its stable marketplace detail, the card reflects the new installed state, and Manage opens /skills/$name for the installed skill.
entry_points: /marketplace/skill/$entryId; skill card Install or Update action
qa_status: pass
bug_ids: BUG-20260714-keyboard-focus-invisible
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38
evidence: /Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/browser-e2e
last_report: docs/qa/reports/2026-08-25-skill-sources.md
overlaps: ET-009; ET-010; ET-api-marketplace-namespace
---

Added by marketplace Task 06. Use an entry whose display name differs from `entry_id` and install slug so detail identity, mutation identity, and Manage routing cannot collapse into one accidental field.

Historical QA note: mutation-result identity routing remains pending.

QA impact 2026-07-16: the success CTA now targets the canonical skill name returned by the install
mutation instead of catalog display metadata; reset with deliberately different names.

QA impact 2026-08-25 (skill sources): reset because the installed skill card changed. It now renders a neutral mono origin pill for skills absorbed from a non-Compozy source, and the kind page's skill query is scoped to the exact profile instead of the remembered one. Re-walk install and update from the stable detail, confirm the card still reflects the new installed state with the origin pill present only for absorbed skills, and confirm Manage still routes to the installed skill. Rides along in `CH-skill-expose-web-repair`.
