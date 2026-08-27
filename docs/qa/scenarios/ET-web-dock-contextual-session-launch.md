---
id: ET-web-dock-contextual-session-launch
area: ET
title: Launch or focus a session from the dock
persona: Bruno
journey: J-operate-desktop-shell
expected: Clicking Sessions in the dock opens the new-session flow only when the workspace catalog is empty and otherwise opens the last created live session, focusing an existing window for that session when one is already open including minimized, off-desktop, or inactive-stack-tab windows.
entry_points: web dock Sessions
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-session-empty-dock-20260827-165738-773687-lab/qa-artifacts/qa/screenshots/dock-last-created.png;/Users/pedronauck/dev/qa-labs/compozy-session-empty-dock-20260827-165738-773687-lab/qa-artifacts/qa/screenshots/dock-empty-create.png;docs/qa/evidence/2026-08-27-session-empty-dock/CH-session-empty-and-dock-last-created-dock-last-created.png
last_report: docs/qa/reports/2026-08-27-session-empty-dock.md
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
