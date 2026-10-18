---
id: MS-web-settings-providers-redesign
area: MS
title: Providers page toolbar, status copy, and provider cards
persona: Dora
journey: J-22
expected: The Providers settings page renders ready/needs-setup/not-installed meta counts, a toolbar (provider search, Status filter button), and one provider card grid (no Rows/Cards toggle) with the pcard anatomy — glyph, display name + Default badge, status line where "Ready · N models" stays neutral (muted text, neutral dot) and only states needing attention ("Needs setup" / "Needs sign-in" / "Not installed") carry warning or danger color, and the auth summary foot ("Uses your local login" / "Key stored in Vault" / "No sign-in needed") with Edit or Set up action. New provider lives in the window head.
entry_points: web settings window Providers section
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-ms-wave2-current-20260730-061842-796290-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: MS-058; MS-web-settings-takeover-redesign
---

Introduced by the opendesign settings redesign (docs/design/opendesign/settings/settings-providers.html, implemented 2026-07-21). Visual contract evidence: .compozy/tasks/os-shell/evidence/visual/opendesign-redesigns/VC-S2/.

2026-09-28 qa-impact (ui-normie-pass settings): Rows/Cards toggle and the mono launch command were removed from the list (the command stays in the provider detail's Technical details); a filter with no matches shows "No providers match" with a "Clear filters" action. Status remains blocked-verify until re-walked.
