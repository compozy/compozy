---
id: APP-web-update-indicator
area: APP
title: Notice an available update from the menubar and land on the Updates section
persona: Ada
journey: J-desktop-agent-headless
expected: A discreet menubar indicator exists only while the daemon reports an available update and no operation is running; it carries no count and opens no menu, disappears through applying, staged, and failed, and activating it by pointer or keyboard opens Settings → General with the Updates section in view.
entry_points: http://127.0.0.1:2123/; GET /api/settings/update
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: APP-web-update-two-track; APP-app-auto-update
---

Added 2026-08-16 for the Electron shell web update surface (ADR-006 S2). Task_07 owns the walk.

PRD stories: US-029 (AC-1 available offer, AC-2 hidden by default, AC-4 keyboard activation; EC-4
suppressed while an operation runs). Test ids: UT-049–UT-052, E2E-021, E2E-022.

Branches to walk:

- nothing offered — the indicator is **absent from the DOM**, not hidden with CSS (a CSS-hidden
  control would still be tab-reachable);
- one track available, then both — the same single visual state either way: no count, no badge, no
  dropdown, no pulse;
- an operation running, staged, or failed — the indicator stays gone; the menubar never renders
  progress, percentages, or errors;
- pointer activation — lands on `/settings/general` with the Updates section visible;
- keyboard-only — the indicator is reachable by Tab between the ⌘K chip and the settings cog, shows
  a visible focus ring against the menubar glass, and activates with Enter or Space.

Walk this in a plain browser as well as the desktop app: the indicator is fed by daemon truth, so
both must behave identically.
