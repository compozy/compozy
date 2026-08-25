---
id: ET-web-dock-contextual-session-launch
area: ET
title: Launch or focus a session from the dock
persona: Bruno
journey: J-operate-desktop-shell
expected: Clicking Sessions in the dock opens the new-session flow when no session window exists and focuses the most-recent session window when one or more session windows already exist, including minimized, off-desktop, or inactive-stack-tab windows.
entry_points: web dock Sessions
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-24-eng-136/cold-new-session.png; docs/qa/evidence/2026-08-24-eng-136/focused-existing-session.png
last_report: docs/qa/reports/2026-08-24-eng-136.md
overlaps: ET-web-sessions-catalog-modal; ET-web-desktop-shell-lifecycle
---

story: As a builder, I can use the Sessions dock icon as a direct launch or return-to-session control without passing through the catalog.

qa-impact: New ENG-136 behavior. The Session menu and palette remain the dedicated catalog controls; the dock action must preserve the current session window's workspace and focus truth.

QA completion 2026-08-24: E2E-136 opened the cold new-session dialog with `general` selected, then restored and focused a minimized session window from the dock. Evidence: `docs/qa/evidence/2026-08-24-eng-136/cold-new-session.png` and `docs/qa/evidence/2026-08-24-eng-136/focused-existing-session.png`. Verdict: pass.
