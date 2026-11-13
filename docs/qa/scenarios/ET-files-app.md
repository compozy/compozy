---
id: ET-files-app
area: ET
title: Browse a project in the Files app with project and root pickers
persona: Dora
journey: J-operate-desktop-shell
expected: The dock shows Files right after Agents; the dock, the palette's "Open project files", every project menu's "Open files" (session list group header, Kanban swimlane, overview tile, Settings project row, menubar project row), a transcript path and Changes' "Open in Files" all open or focus one Files window per project (from a session, as a tab of its frame, at the file and line); the head reads Files then the pickers, and the tab names the open file; the root picker switches between the project root and ready worktrees; the project switcher lists live projects; a removed worktree root shows the unavailable state with "Switch to project root"; Markdown, SVG, CSV/TSV, HTML, Mermaid and notebooks open rendered with Source / Preview / Split one click (or Cmd/Ctrl+Shift+V) apart and the mode remembered per kind, while PDF, video, audio and images over 5 MB play or draw from the daemon's raw-bytes route.
entry_points: OS dock Files; command palette Open project files; project menu Open files (sessions rail group header, Kanban swimlane header, overview tile, Settings project row, menubar project row); transcript file path plain click; Review panel Changes Open in Files; /files/<workspace_id>
qa_status: pending
bug_ids: BUG-20261107-files-root-internal-workspace-id
fix_status: fixed
retest_status: pending
fix_commits: c5d1d54a6
evidence: lab compozy-file-explorer-integration-20261107-073905-206925 (qa-artifacts/qa/journey-log.jsonl, screenshots/A01–A07)
last_report: docs/qa/reports/2026-11-07-file-explorer-integration.md
overlaps: ET-transcript-paths-files-app; ET-files-search
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
8. From a session, a transcript path's plain click and Changes' **Open in Files** open the project's
   Files window as a tab of the session's frame on the session root, at the file and line, or
   retarget the existing one (see `ET-transcript-paths-files-app` steps 2 and 3).
9. Deep, long paths (#1480): open a file six levels deep with **Open in Files** and check the tree
   at the default width and in a narrow window. Long names truncate behind one clean "…" with
   their extension, a single-child chain shows as one `proto / review` row that reveals through,
   one click on a folder whose only child is a folder opens both, and hovering a row shows its full
   path.
10. Previews (#1605): open `docs/guide.md` (or any Markdown with a table, a Mermaid block, a
    relative link and a relative image). It opens in **Preview**: headings, the table and the
    diagram render, frontmatter shows as a YAML block, the relative image loads, and the relative
    link opens that file in the same window. **Source** shows the raw text; **Split** shows both
    side by side; Cmd/Ctrl+Shift+V flips Source and Preview, and Cmd/Ctrl+F in Preview switches to
    Source with Find open. Close and reopen a Markdown file: the last mode is kept. A file opened at
    a line (Search, Go to file with `:line`, a transcript path) opens in Source at that line.
11. Repeat Preview for a `.csv`/`.tsv` (table with a sticky header and row numbers), an `.svg`
    (image; Source shows the markup), an `.html` page (rendered without running its scripts; its
    relative CSS and images load; a link to another project file opens it in Files), a `.mmd`
    diagram and an `.ipynb` notebook (cells, prompts and saved outputs). A Markdown file over 1 MB
    shows only Source with the truncation note.
12. Open a `.pdf` (pages render and scroll), an `.mp4`/`.webm` (plays and seeks), an `.mp3` and an
    image larger than 5 MB: each streams from the daemon. Repeat one from a paired device. A `.env`
    stays gated: its raw route answers 403.

Persistence: the root and the open path persist per project under `compozy:files:v1:<workspace_id>`;
the view mode persists per preview kind under `compozy:files:viewer-mode:v1`.

Owning automated coverage: web `app-registry.test.ts` (UT-045, UT-046), `files-app-window.test.tsx`
(UT-047, US-002.AC-6), `file-viewer-pane.test.tsx` (Preview suite, #1605), `os-dock.test.tsx`, `session-list.test.tsx`; Go `provider_test.go` (palette
command) and `TestWindowManagerCrossPartitionRetirement` (IT-012); `web/e2e/__tests__/files-explorer.spec.ts`
E2E-002.

## Walk 2026-11-07

Same lab and build as `ET-transcript-paths-files-app` (integration `410a3566e`, fix `c5d1d54a6`), with
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
| 8 Open in Files window | pass after fix | See `ET-transcript-paths-files-app` step 8 and BUG-20261107-files-root-internal-workspace-id |
| 9 Deep paths (2026-11-10, #1480) | pass | Lab `compozy-issue-1480-file-tree-20261111-001512-265025`, fix `d12f6acbb`: before `admuustomer-detail`, `:payment-methods` (`05-files-tree-before-default.png`, `07-files-tree-before-narrow.png`); after `admin-billing…ustomer-detail`, revealed `admin-billing-subscri…tsx` under `proto / review` (`06-files-tree-after-default.png`, `08-files-tree-after-narrow.png`) |
