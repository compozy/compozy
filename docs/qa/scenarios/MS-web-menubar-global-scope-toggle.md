---
id: MS-web-menubar-global-scope-toggle
area: MS
title: Menubar Global toggle owns destination
persona: Bruno
journey: J-operate-desktop-shell
expected: A 28px Globe toggle sits between the CompozyOS mark and the workspace chip, outside `role="menubar"`. Off is muted like the bell; on is pressed fill plus accent globe (`aria-pressed`). ON sets the chip to Global (`~`) and keeps the remembered project id; OFF restores that project when it still exists. ⇧⌘G toggles the same control and is skipped on editable targets. With zero project folders the toggle stays on and is `aria-disabled` (not `disabled`) with tooltip "Add a workspace to scope down"; with project folders but no remembered selection it stays on with tooltip "Pick a workspace to scope down" (never the add-a-workspace copy). While the workspace catalog is still loading the toggle claims nothing — no locked reason. A polite live region announces the mode. The workspace menu lists project folders only; while Global is on it shows no check and no info or warning notice; picking a folder turns Global off. Compact viewports keep logo · globe · chip leading after app menus hide.
entry_points: web desktop menubar; ⇧⌘G; command palette Turn on/off Global scope
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-pr-368-coderabbit-20260813-051821-831054-lab/qa-artifacts/qa/screenshots/scope-project-tmp.png; /Users/pedronauck/dev/qa-labs/compozy-pr-368-coderabbit-20260813-051821-831054-lab/qa-artifacts/qa/screenshots/scope-global.png
last_report: docs/qa/reports/2026-08-13-pr-368-coderabbit.md
overlaps: ET-web-menubar-menu-set; ET-web-command-palette-shortcuts; MS-web-workspace-lists-hide-home
---

story: As a builder I flip one menubar globe to work across every project folder, and I always know which destination create dialogs will use.

Introduced 2026-08-12 by menubar-owned Global scope. Persist key `compozy:active-workspace:v3` stores `scope` plus `selectedWorkspaceId`. Empty v3 hydrates as Global. `$HOME` is not a UI row.

src: web/src/systems/os/components/global-scope-toggle.tsx; web/src/systems/os/components/desktop-menubar.tsx; web/src/systems/os/components/os-menubar.tsx; web/src/systems/os/components/menubar/workspace-menu.tsx; web/src/systems/workspace/stores/active-workspace-store.ts; web/src/systems/workspace/lib/active-workspace.ts

2026-08-20: Deletion notice removed from the workspace menu. The chip stays Global when a remembered folder is gone; the menu lists remaining folders with no info or warning banner. Walk this cycle: blocked-verify — workspace-menu unit suite asserts no note/alert in the menu; an isolated QA lab with a live daemon was not started.

2026-08-17: Global-on info notice removed from the workspace menu and the workspaces overview. The chip, globe, and live region already name the mode; the deletion notice stays when a remembered folder is gone. Walk this cycle: blocked-verify — workspace-menu and workspaces-overview unit suites assert the notice is gone and picking still works; an isolated QA lab with a live daemon was not started.

2026-08-13: Globe moved between the mark and the workspace chip (leading cluster). Expected placement updated; previous captures that show a trailing globe need a re-capture. Walk this cycle: blocked-verify — unit tests for the toggle passed; an isolated QA lab with a live daemon was not started, so a persona walk through public entry points could not meet the qa-execution evidence standard.

2026-08-13: labeled Switch replaced by a 28px Globe Toggle (`aria-pressed`, accent when on). Expected control updated; previous Storybook captures (VC-01–VC-04) show the Switch and need a re-capture.

2026-08-12 fix pass: honest locked tooltips (pick vs add), keyboard-reachable tooltip, visible disabled affordance, announce-on-change live region, and loading-vs-deleted separation landed after the visual capture; the walk below predates them.

2026-08-12 walk: blocked-verify. This implementation cycle captured Storybook visual-contract evidence (`.compozy/tasks/global-workspace-menubar/evidence/visual/menubar-toggle/VC-01`–`VC-04`) and unit/typecheck coverage. An isolated QA lab with a live daemon (`COMPOZY_HOME`, production-parity web) was not started, so a persona walk through public entry points could not meet the qa-execution evidence standard.

2026-08-13 re-walk: Bruno switched from project `tmp` to Global through the globe, confirmed the project stayed available, then used the command palette action "Switch to tmp turns Global scope off". Refresh preserved `tmp`; the project menu never exposed the operator-home registration.
