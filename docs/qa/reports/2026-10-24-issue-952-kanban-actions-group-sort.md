# Issue #952 — Kanban card actions, board group/sort, per-window order

Date: 2026-10-24. Build: PR #963 at `e16bcd082f99` (web from the worktree via Vite, daemon built from the same tree).
Lab: isolated `eng-qa-bootstrap` targeted lab (web, api); `scripts/demo-seed` (Northstar Pay, 2 projects) plus
six acpmock sessions across both projects and the `default` and `open-design` profiles (working / done; the seeded
needs-you child). Teardown clean.

## Walk — ET-web-sessions-board-card-actions-group-sort: PASS

| Step | Observed | Evidence |
| --- | --- | --- |
| Session window head | `List \| Kanban` beside the title, no per-session `…` | [01](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-952-kanban/e16bcd082f99/01-list-sort-menu.png) |
| List toolbar | Two icons; Sort by menu (5 sorts) and Group by menu (None/Project under All projects) | [01](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-952-kanban/e16bcd082f99/01-list-sort-menu.png), [02](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-952-kanban/e16bcd082f99/02-list-group-by-menu.png) |
| Own row menu | Clear conversation present, disabled while the turn runs; enabled on an idle session and opens the clear confirmation; absent on other rows | [03](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-952-kanban/e16bcd082f99/03-own-row-menu-clear-conversation.png), [03b](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-952-kanban/e16bcd082f99/03b-clear-conversation-confirm.png) |
| Kanban head | Group by, Sort by before Hide child sessions | [04](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-952-kanban/e16bcd082f99/04-kanban-header-group-sort.png) |
| Card `…` | Revealed on hover; row verbs; session not opened | [05](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-952-kanban/e16bcd082f99/05-kanban-card-dots-menu.png) |
| Card right-click | `@compozy/ui` context menu with the row verbs, no native Copy menu | [06](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-952-kanban/e16bcd082f99/06-kanban-card-context-menu.png) |
| Group by Project | One swimlane per project; lane counts equal the sum of its column counts (7 = 0+1+0+6, 9 = 1+1+0+7); cards omit the project the lane names | [07](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-952-kanban/e16bcd082f99/07-kanban-grouped-by-project.png) |
| Sort by Name + All profiles + Group by Profile | `default` and `open-design` lanes; Done ordered by name | [08](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-952-kanban/e16bcd082f99/08-kanban-grouped-by-profile.png) |
| Back to List | List grouped by profile, sorted by name — the window's choice carried across the toggle | [09](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-952-kanban/e16bcd082f99/09-list-keeps-window-group-and-sort.png) |
| Fresh browser context | Starts from the daemon seed (Created / None) — per-window state is browser-local, never written to `shell.sessions` | script log |

No console errors in any capture. Automated owners are listed in the scenario.
