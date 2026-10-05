---
id: ET-web-dock-contextual-session-launch
area: ET
title: Launch or focus a session from the dock
persona: Bruno
journey: J-operate-desktop-shell
expected: Clicking Sessions at the top of the dock (the left rail) opens the new-session flow only when the workspace catalog is empty and otherwise opens the last created live session, focusing an existing window for that session when one is already open including minimized, off-desktop, or inactive-stack-tab windows; the Sessions item carries the active plate while that window is focused and its badge names what needs you ("Sessions — 1 needs you").
entry_points: web dock Sessions (left rail) — click, ⌥-click, ⇧-click, right-click menu
qa_status: skipped
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/Dev/qa-labs/compozy-reported-issues-20260929-20260930-025651-581547-lab/qa-artifacts/qa/final-dock-restored.json; docs/qa/reports/2026-09-29-reported-issues.md
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-web-sessions-catalog-modal; ET-web-desktop-shell-lifecycle
---

story: As a builder, I can use the Sessions dock icon as a direct launch or return-to-session control without passing through the catalog.

qa-impact: New ENG-136 behavior. The Session menu and palette remain the dedicated catalog controls; the dock action must preserve the current session window's workspace and focus truth.

QA completion 2026-08-24: E2E-136 opened the cold new-session dialog with `general` selected, then restored and focused a minimized session window from the dock. Evidence: `docs/qa/evidence/2026-08-24-eng-136/cold-new-session.png` and `docs/qa/evidence/2026-08-24-eng-136/focused-existing-session.png`. Verdict: pass.

QA impact 2026-08-27: the Sessions dock icon now keys off catalog emptiness and `created_at`, not
open session windows. Reset for a walk that covers empty catalog → create, seeded session without a
window → last created, and last-created already open → focus.

QA execution 2026-08-27: with a catalog row and no window for it, Sessions opened Dock last created
instead of the create modal and instead of leftover empty /sessions windows. After the catalog went
empty, Sessions opened create with general selected. Detached Plus still opened create while a
session was focused. Verdict: pass.

QA re-walk 2026-09-30: PASS on actual isolated Electron final Web index71ea3236e585a54471d994843a07ea687a1797a34badc28b6af7086f13d18b57. A newly registered empty project opened the real Start session form from Sessions; Cancel preserved the empty catalog. Public CLI then created Dock external newest session, and the next actual dock click opened that exact session instead of another create form. Actual window Minimize followed by dock click restored the same session, visible and focused. The cache was empty before external creation; its value at click was not independently observed. Unchanged remote E2E-136 covers the precise stale-cache regression. Evidence: final-dock-cold-workspace.json, final-dock-external-new.json and final-dock-restored.json in the canonical report lab.

qa-impact: 2026-09-30 shell rail v2 (flat topbar, left dock rail, gutterless tiling, browser-tab deck, light/dark theme). The dock moved from the bottom glass strip to the 60px left rail (same launch rules, new component). Reset for a walk of the three cases: empty catalog → create, catalog row without a window → last created, last-created already open (including minimized) → focus.

qa-impact: 2026-09-30 shell-rail polish P6 — plain Sessions clicks keep the three cases above; a last-created session with no window now opens as a tab of the focused window (daemon default `tab`). New: Option (⌥)-click, Shift (⇧)-click, and the Sessions right-click menu (Open in new tab / Open in split / Open in new window / Open in new desktop / Go to tab) open the session list window (`/sessions`, "No session selected") at that destination instead of jumping to the latest session; in Global scope with no sessions every gesture still opens the workspace switcher. Reset for a walk of those gestures.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
