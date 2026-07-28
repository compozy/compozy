---
id: MS-web-settings-takeover-redesign
area: MS
title: Settings takeover shell with srow pages and save models
persona: Dora
journey:
expected: The settings window renders the 264px takeover sidebar (Back to app closes the window; search with `/` shortcut filters sections; Workspace/Runtime/System groups; daemon foot) collapsing to a chip strip under 56rem. Pages use one-decision srows with consequence sentences, at most one Advanced fold per page, and choice cards with neutral selection. Draft pages show the floating save bar only when dirty/saving/error and flash "Saved" after a clean save; restart-needed changes surface the typed restart notice.
entry_points: web settings window (General, Memory, Automation, Skills, Hooks, Extensions, Network, Observability)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-026; MS-037; ET-012; ET-044; ET-045
---

Introduced by the opendesign settings redesign (docs/design/opendesign/settings/settings-general.html and siblings, implemented 2026-07-21). Visual contract evidence: .compozy/tasks/os-shell/evidence/visual/opendesign-redesigns/VC-S1/.
