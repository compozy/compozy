---
id: MS-web-stays-on-placement
area: MS
title: Stays-on agents sit as avatars above sessions and in Kanban headers
persona: Dora
journey: J-31
expected: With no instance there is no row and no empty space. Global instances sit in a row labeled Global above all projects; each project's instances sit under its name, and in its Kanban lane header. Anyone needing the operator is ordered first and shows the Needs you triangle. A click opens the main session, created again if it was lost, and the open one shows a selected background. Hover or focus opens a card with what it's doing, last and next check, Open conversation and Pause or Resume. The context menu offers Pause or Resume with the pause reason, Open agent, Check-ins and Archive; Resume is disabled when the definition is gone. More agents than fit move into `+N`, a searchable panel grouped by state. A collapsed project shows who needs you by face. Main sessions leave session rows, cards and counts. Sessions they spawn stay ordinary rows and cards with a creator badge. A project with only an agent still gets its group. A project's `+` offers New session, New folder and Add stays-on agent; Add opens a picker of stays-on agents not yet there, with Create new opening New agent on Stays on with that project chosen. At the limit the rows and Create new are blocked and the popover shows it. Flat mode shows one row: Global first, then the focused project's. The dock's Home app reads Dashboard.
entry_points: web Sessions window (list and Kanban); project `+`; avatar row; avatar context menu
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-web-agent-create-simple-advanced
---

Needs the instance routes from the stays-on backend slices. Create a stays-on agent in a project and
one Global, then walk the row, its card and menu, the picker, Kanban and the collapsed signal. Pause
and resume from the menu, archive and confirm the session returns to history, and watch
`agent_instance.updated` move a pose without a reload.

QA impact 2026-11-01 (stays-on F1): built against the `_dx.md` contract with typed adapters and mocks,
before those routes merged. Focused suites own the states. Not walked.
