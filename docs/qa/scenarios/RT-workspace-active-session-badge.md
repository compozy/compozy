---
id: RT-workspace-active-session-badge
area: RT
title: Surface background sessions across workspaces
persona: Théo
journey: J-11
expected: Switching away from a workspace leaves its live session running, shows an exact active-session count on that workspace, surfaces a catalog-read warning instead of reporting a false zero, and lets the operator reopen the background session with its current transcript.
entry_points: web workspace switcher; web session list; web session permalink
qa_status: untested
bug_ids: BUG-20260713-background-session-indicator-title;BUG-20260713-cross-workspace-session-return-hangs
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/rt-agh84-two-workspace-badge-title.dom.txt;/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/rt-agh84-cross-workspace-return-hang.dom.txt;/Users/pedronauck/dev/qa-labs/agh-automation-features-final-replay-20260713-20260713-194432-535561-lab/qa-artifacts/qa/screenshots/rt-agh84-onboarding-session-counted-as-user.dom.txt;/Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/screenshots/rt-agh84-onboarding-system-badge-one-fixed.dom.txt;/Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/screenshots/rt-agh84-cross-workspace-return-fixed.dom.txt
last_report: docs/qa/reports/2026-07-13-automation-features.md
overlaps: RT-041;RT-045
---

Linear issue AGH-84 is the named regression target.

QA impact 2026-07-14: per-workspace session query failures now render a warning state instead of an idle count. Planning update only; reset to untested without a QA replay.
