---
id: ET-transcript-paths-files-app
area: ET
title: Open transcript file paths in the project's Files app
persona: Dora
journey: J-14
expected: A plain click on a file path in an assistant message opens the project's Files app as a tab of the session's window frame (the session keeps its tab and is not navigated), on the session root (bound worktree, else checkout), with the file in the viewer at its line and the tree revealing it; a second path retargets the same Files window instead of opening another; the Review panel does not open; every viewer state (sensitive gate, image, binary, truncated, not found, outside this project) renders; a User-scope session renders plain code with no Review panel; unloaded changed folders show a dot, tree rows have a Copy path menu, image previews show their dimensions, and the target line is visible in both themes.
entry_points: web session window transcript (inline code paths in assistant messages); Files app tab opened from it
qa_status: pending
bug_ids: BUG-20261107-files-root-internal-workspace-id
fix_status: fixed
retest_status: pending
fix_commits: c5d1d54a6
evidence: renamed from ET-transcript-paths-files-app when the Review panel lost its Files tab (fix/files-window-head); automated coverage below passes, the manual walk is pending
last_report: docs/qa/reports/2026-11-07-file-explorer-integration.md
overlaps: APP-open-transcript-paths-in-editor; ET-session-review-changes; ET-files-app
---

Introduced by the file-explorer spec (US-001, US-003–US-005, US-012) as `ET-transcript-paths-files-app`,
when transcript paths opened the Review panel's Files tab. That tab is gone: transcript paths now
open the project's Files app. Walk it on a real daemon with a git project, a session whose agent
replies with file paths, and a browser tab (no desktop bridge).

Seed the project with a text file of at least 12 lines, a `.env`, a PNG, a binary file, a text file
of several thousand lines, and a home-relative path outside the project (for example
`~/.ssh/config` under a fake `HOME`).

Walk:

1. Open the session. Have the agent reply with `` `src/main.go:12` `` and the other paths. Each path
   renders as a link button named "Open <name> in Files", with the tooltip "Open in Files".
2. Plain-click `src/main.go:12`. The project's Files window opens as a new tab in the session's
   window frame and becomes the active tab; the session's tab stays in the strip and its window is
   not navigated. The Files head reads **Files** with the project and root pickers; the tab reads
   `main.go`. The viewer header shows `src/main.go`, line 12 is highlighted, and the tree reveals
   the file with its git status letters. The Review panel does not open.
3. Back on the session tab, click `.env:1`. The same Files tab (no second Files window) shows
   "Sensitive file" with **Show contents**. Clicking it reveals the text.
4. Click the PNG: the image renders. Click the binary file: "<size> · No preview". Click the long
   file at `:4800`: the viewer opens at line 4800.
5. Click a missing path: the not-found state names the path and offers to refresh the tree. Click
   `~/.ssh/config`: "Outside this project". In a browser tab, the viewer header has no Open in
   control and no "Choose application" button.
6. Bind the session to a worktree and click a path: the Files root picker shows that worktree.
7. Open a User-scope session whose agent cites the same paths. They render as plain inline code,
   and there is no Review panel toggle.
8. Tree and viewer polish: a collapsed folder that contains changes shows the tree's dot before it
   is expanded. Right-click a tree row (or Shift+F10): the menu offers **Copy path**, plus the Open
   in targets in the desktop app. Open a PNG: the preview caption reads
   "<width> × <height> · <size>". Open `file:200` in light and dark themes: the target line is
   clearly tinted.

Owning automated coverage: web `file-path-links.test.ts` (UT-048–UT-051), `message-markdown.test.tsx`
(UT-041, UT-042, UT-054: the Files open on the session root and line), `files-app-window.test.tsx`
(US-002.AC-6 re-root and the outside target), `file-viewer-pane.test.tsx` (UT-055 and the viewer
states), and `web/e2e/__tests__/files-explorer.spec.ts` E2E-001 and E2E-003. The root's
`workspace_id` contract is owned by `TestProjectFileContentReveal` (core) and
`TestNativeSessionChanges` (daemon).

## Walk 2026-11-07 (superseded: Review panel Files tab)

This walk ran against the earlier flow, where paths opened the Review panel on Files.

Build: `feat/file-explorer-s10` on integration `410a3566e`, daemon and `web/dist` built from the
worktree, re-walked on `c5d1d54a6` after the fix. Isolated targeted lab (`eng-qa-bootstrap`, own
`COMPOZY_HOME`, port 52129 and socket), fake `HOME`, and the acpmock driver as the agent. Driven
with Playwright Chromium at 1600×1000.

| Step | Result | Evidence |
| --- | --- | --- |
| 1 Links render | pass | `F01-transcript-links.png` |
| 2 Plain click → viewer at line 12, Inspector closed | pass | Files tab selected, `data-highlighted-line=12`, Inspector count 0 (`F02-viewer-main-go-12.png`) |
| 3 Sensitive gate → Show contents | pass | `F03-sensitive-gate.png`, `F03b-sensitive-revealed.png` |
| 4 Image, binary, line 4800 | pass | `F04-image.png`; "50 KB · No preview" (`F05-binary.png`); 6,000 lines, line 4800 highlighted (`F06-long-4800.png`) |
| 5 Not found, outside, no Open in in a browser | pass | `F07-not-found.png`, `F08-outside.png`; zero Open in or Choose application controls |
| 6 Tree expand and open | pass | `F09-tree-click.png` |
| 7 Persistence after reload | pass | stored `{"open":true,"tab":"files"}` |
| 8 Open in Files window | fail → fixed → pass | Before the fix, the window opened keyed by the internal project ID and read "Project not found". After `c5d1d54a6`, it is keyed `ws_e72b1865b5dccbd5` with `README.md` open (`F12-open-in-files-window-fixed.png`) |
| 9 User scope | pass | Plain code, no visible Review toggle (`F13-user-scope.png`) |

BUG-20261107-files-root-internal-workspace-id: file, search and changes responses (HTTP/UDS and
`compozy__session_changes`) set `root.workspace_id` to the project's internal identity instead of
the public registry ID. Fixed in `c5d1d54a6`.

## Backend safety regression checks

- Create a named pipe in the project without attaching a writer. Content reads must promptly return
  `422 not_a_file`; listing that path must promptly return `422 not_a_directory`.
- Cancel a request before opening a path. It must return cancellation without starting a read.
- Replacing a regular path with a special file during opening must never block the daemon.
  Descriptor validation still runs after the confined, nonblocking open.

The existing `TestReader` suite owns special-file rejection. Windows compilation validates the
platform-specific open flags; it does not constitute a Windows runtime walk.

- On filesystems that support trailing spaces, create both `notes.txt` and `notes.txt ` with
  different content. Each tree entry must retain its exact name and return its own content.
  Repeat with only `notes.txt ` present. Windows rejects these unsupported names instead of
  opening a different file.

## Backend status snapshot checks

Sibling expansions share one root-scoped status snapshot for one second. Explicit Refresh, folder
Retry and change-event refreshes send `Cache-Control: no-cache`, bypassing that snapshot through
the shared HTTP/UDS handler. Verify a new working-tree change appears after that request.
Directory ordering and ignored-file filtering still precede the 5,000-entry response cap.

For overlapping status requests, disconnect the first client while Git is active. A remaining
client must obtain status through a new shared attempt, rather than inherit the disconnected
client's error. Canceling all clients must stop Git work.
