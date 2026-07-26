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
fix_status: BUG-20260714-keyboard-focus-invisible fixed
retest_status: pending mutation-result identity routing
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-under-minute.json; /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/web/marketplace-skill-manage.png; /Users/pedronauck/Dev/compozy/agh/.tmp/bug-20260714-focus/focused.png
last_report: docs/qa/reports/2026-07-15-marketplace.md
overlaps: ET-009; ET-010; ET-api-marketplace-namespace
---

Added by marketplace Task 06. Use an entry whose display name differs from `entry_id` and install slug so detail identity, mutation identity, and Manage routing cannot collapse into one accidental field.

QA impact 2026-07-16: the success CTA now targets the canonical skill name returned by the install
mutation instead of catalog display metadata; reset with deliberately different names.
