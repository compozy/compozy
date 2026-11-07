---
id: ET-files-app
area: ET
title: Browse a project in the Files app with project and root pickers
persona: Dora
journey: J-operate-desktop-shell
expected: The dock shows Files right after Agents; the dock, the palette's "Open project files", the project context menu's "Open files" and the Review panel's "Open in Files window" all open or focus one Files window per project; the root picker switches between the project root and ready worktrees; the project switcher lists live projects; a removed worktree root shows the unavailable state with "Switch to project root".
entry_points: OS dock Files; command palette Open project files; sessions rail project group header context menu Open files; Review panel Files tab Open in Files window; /files/<workspace_id>
qa_status: pass
bug_ids: BUG-20261107-files-root-internal-workspace-id
fix_status: fixed
retest_status: pass
fix_commits: c5d1d54a6
evidence: lab compozy-file-explorer-integration-20261107-073905-206925 (qa-artifacts/qa/journey-log.jsonl, screenshots/A01–A07)
last_report: docs/qa/reports/2026-11-07-file-explorer-integration.md
overlaps: ET-session-review-files; ET-files-search
---

Introduced by the file-explorer spec (US-002; `_uiux.md` S6, S10). Walk it with two registered
projects, one of them a git repository with a ready worktree.

Walk:

1. The dock order runs … Agents, **Files**, Tasks …. Click Files: a Files window opens on the active
   (or remembered) project. The topbar shows the project switcher and the root picker, with the
   branch.
2. Open the root picker: it lists **Project root** and every ready worktree with its branch. Choose
   the worktree: the tree re-roots and the branch label changes.
3. Click Files in the dock again: the same window gets focus, still on the worktree root (one
   window per project).
4. Open the project switcher: it lists live projects only.
5. Close the window. In the command palette, run **Open project files**: exactly one Files window
   opens.
6. Group the sessions rail by project (View options › Group by › Project). Right-click a project's
   group header and choose **Open files**: that project's Files window opens.
7. With the Files window on a worktree root, remove the worktree (`compozy worktree remove`) and
   reload. The window shows the root-unavailable state; **Switch to project root** returns to the
   checkout.
8. From a session's Review panel, **Open in Files window** opens or focuses the same project window
   (see `ET-session-review-files` step 8).

Persistence: the root and the open path persist per project under `compozy:files:v1:<workspace_id>`.

Owning automated coverage: web `app-registry.test.ts` (UT-045, UT-046), `files-app-window.test.tsx`
(UT-047, US-002.AC-6), `os-dock.test.tsx`, `session-list.test.tsx`; Go `provider_test.go` (palette
command) and `TestWindowManagerCrossPartitionRetirement` (IT-012); `web/e2e/__tests__/files-explorer.spec.ts`
E2E-002.

## Walk 2026-11-07

Same lab and build as `ET-session-review-files` (integration `410a3566e`, fix `c5d1d54a6`), with
projects `lab-project` (worktree `feature-lab`) and `bulk-project`.

| Step | Result | Evidence |
| --- | --- | --- |
| 1 Dock order and open | pass | Dock "Terminal · Agents · Files · Tasks"; root `lab-project · feat/review` (`A01-files-app.png`) |
| 2 Root picker | pass | Options "Project root" and "calm-badger feature-lab"; re-rooted to `feature-lab` (`A02-worktree-root.png`, `A02b-root-picker.png`) |
| 3 One window per project | pass | One window, root kept on `feature-lab` |
| 4 Project switcher | pass | Lists `bulk-project` and `lab-project` (`A04-project-switcher.png`) |
| 5 Palette | pass | "Open project files" opened one window (`A05-palette.png`) |
| 6 Project context menu | pass | **Open files** opened the `bulk-project` window (`A06-context-menu.png`, `A06b-bulk-files.png`) |
| 7 Removed worktree | pass | Root unavailable, then Switch to project root restored `feat/review` (`A07-root-unavailable.png`) |
| 8 Open in Files window | pass after fix | See `ET-session-review-files` step 8 and BUG-20261107-files-root-internal-workspace-id |
