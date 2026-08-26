---
id: ET-profile-switcher-restore
area: ET
title: Create, switch, and restore profiles from the menubar switcher
persona: Ada
journey: J-operate-profiles
expected: The switcher is a neutral icon button while only default exists, becomes an identity element once a second profile is created, switches through the canonical selection route, answers the boundary question in one sentence, offers the All-profiles state, and restores each project's remembered profile on return without ever force-switching an already-open client.
entry_points: menubar profile switcher; Create profile… dialog; command palette Profiles view; profile.use; GET|PUT /api/profiles/selection; GET /api/logs/stream?component=profile
qa_status: pass
bug_ids:
fix_status: not-needed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-profiles-final-20260826-081429-551001-lab/qa-artifacts/qa/quiet-profile-trigger.png; /Users/pedronauck/dev/qa-labs/compozy-profiles-final-20260826-081429-551001-lab/qa-artifacts/qa/global-profile-restored.png; /Users/pedronauck/dev/qa-labs/compozy-profiles-final-20260826-081429-551001-lab/qa-artifacts/qa/workspace-profile-restored.png; /Users/pedronauck/dev/qa-labs/compozy-profiles-final-20260826-081429-551001-lab/qa-artifacts/qa/all-profiles-layered-mark.png
last_report: docs/qa/reports/2026-08-26-profile-identity-final.md
overlaps: ET-profile-selection-precedence; ET-profile-palette-view; MS-web-menubar-global-scope-toggle
---

Flagged by Profiles task 05. The final QA tasks own the real-user walk, evidence, and verdict.

Walk:

1. On a fresh home, confirm the switcher renders as a quiet icon button with no name or color, and
   that nothing else in the product mentions profiles.
2. Create a second profile from the switcher, choosing an icon and a color in the picker; confirm the
   switcher becomes an identity element showing the exact icon and color that were chosen (never a
   generic placeholder glyph), both on the trigger and on the menu rows, and the new profile is
   active.
3. Switch profiles and confirm listings refilter, the boundary sentence is present verbatim, and the
   project picker lists the same projects in every profile.
4. Leave the project, return, and confirm the remembered profile is restored; repeat for the Global
   lens and confirm it keeps its own slot.
5. With the browser open on one profile, run `compozy profile use <other>` in a terminal. Confirm the
   remembered choice updates in Settings while the open client keeps showing the profile the operator
   was already looking at.
6. Turn on All profiles, confirm the neutral layered mark replaces the identity, then leave and
   return to a project and confirm it lands on a real profile rather than the aggregate.

Expected evidence: screenshots of the quiet and plural states, the switcher menu, selection-route
request/response pairs, terminal transcript for the cross-surface switch, and the restored state after
re-entering each project.

QA 2026-08-26: Passed in an isolated lab. Global and workspace selections restored independently,
the open browser resisted an external CLI switch, and All profiles returned to a real profile after
re-entry.
