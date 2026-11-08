---
id: APP-open-in-targets
area: APP
title: Open a viewed file in a detected desktop application and remember the target
persona: Dora
journey: J-desktop-attach-daily
expected: In the packaged desktop app, the viewer header's Open in split button lists the configured editor (only when desktop.editor_command is set), every detected catalog editor (brand icon where available, generic icon otherwise), Finder (Show in folder / Show in Explorer), Copy path, and a Refresh list / Editor settings footer, or "Set an editor in Settings" when no editor exists; choosing one launches it at the file and line without a shell and remembers it as the primary action; tree rows and the root header offer the same menu for files and folders without a line; Cmd/Ctrl-click on a transcript path opens the remembered target named in its tooltip; outside-project paths and credential files ask first with distinct titles and Cancel launches nothing; a vanished editor is named in the toast and the list refreshes; Finder only reveals; browser tabs show no Open in control and keep Copy path.
entry_points: packaged desktop main window: Files app viewer header (Open in split button, Choose application); tree row context menu and tree header "Open folder in…"; viewer "Outside this project" state; transcript path Cmd/Ctrl-click; Settings › General › Editor command
qa_status: partial
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: desktop/e2e/_electron/__tests__/shell.spec.ts E2E-008 and E2E-001 (#1167) on the S10 head (/tmp/s10/desktop-e2e.log, 2 passed in 20.0 s)
last_report: docs/qa/reports/2026-11-07-file-explorer-integration.md
overlaps: APP-open-transcript-paths-in-editor; ET-transcript-paths-files-app
---

Introduced by the file-explorer spec (US-006, US-011, US-005.AC-4; ADR-003; `_uiux.md` S5). It
replaces the "Cmd-click opens the configured editor only" behavior of `APP-open-transcript-paths-in-editor`.
Walk it once on a packaged desktop app with at least two real editors installed (for example VS Code
and Zed) and `desktop.editor_command` set.

Walk:

1. With `desktop.editor_command` unset, open a file in the Files app viewer. The header shows
   **Open in <first detected editor>** with its icon (or the generic editor icon) and a **Choose application** chevron.
2. Open Choose application. The menu lists every detected application from the catalog (Cursor,
   VS Code, VS Code Insiders, Windsurf, Zed, Sublime Text, IntelliJ IDEA, WebStorm, GoLand, PyCharm,
   Xcode, Android Studio, Fleet), a separator, Finder (macOS), Show in folder (Linux) or Show in
   Explorer (Windows) and **Copy path**, a separator, then **Refresh list** and **Editor settings…**.
   No **Configured editor** entry appears. Applications that are not installed are absent; detection
   reads the login-shell PATH once per app run, plus macOS application bundles.
3. Set `desktop.editor_command`, choose **Refresh list**: **Configured editor** now leads the menu,
   above a separator. On a machine with no editor installed and none configured, the menu shows
   **Set an editor in Settings** instead, and the primary half reads **Reveal in Finder**.
4. Choose an editor. It opens the file at the viewer's line (VS Code-family editors get
   `-g <path>:<line>:<col>`; JetBrains IDEs, Android Studio and Xcode get `--line <line> <path>`;
   Fleet gets the path only). The primary button now reads **Open in <that editor>**, also after a
   reload (`compozy.open-target`).
5. Right-click a folder row in the tree (or press Shift+F10 on it): the menu shows **Copy path** and
   the same targets. Choosing an editor opens the folder with no line. The tree header's
   **Open folder in…** button does the same for the root.
6. In the transcript, hover a path: the tooltip reads "Open in Files · ⌘-click opens in <target>".
   Cmd-click (macOS) or Ctrl-click opens it directly in the remembered target and does not open a
   Files tab. A plain click still opens the Files app tab.
7. Choose **Finder**: the file is revealed and selected, never opened or executed.
8. Open `.env` or a key file inside the project and choose a target: a native sheet titled
   "Open a file that may contain credentials?" appears. A path outside the project shows
   "Open file outside this workspace?" with "It is outside the session's workspace." Cancel
   launches nothing. `.env.example`, `.env.sample` and `.env.template` open without asking.
9. Make the remembered editor unavailable (uninstall it, or remove it from PATH and relaunch). The
   toast reads "<Editor> is no longer available", nothing launches, and the menu no longer lists it.
10. In a browser tab on the same daemon, the viewer has no Open in control, Cmd/Ctrl-click opens
    the Files app tab, and the tree row menu holds only **Copy path**.

Owning automated coverage: desktop `file-opener.test.ts` (UT-060–UT-065), `product-contract.test.ts`,
`product-bridge-controller.test.ts` (main-frame authorization), `login-shell-path.test.ts`; web
`file-viewer-pane.test.tsx` (UT-055, target selection), `open-in-button.test.tsx` (menu structure,
AC-5 empty state, Refresh list, vanished target, tree row menu), `message-markdown.test.tsx`
(UT-052, tooltip target); `packages/ui` `file-tree.test.tsx` (row menu); packaged
`shell.spec.ts` E2E-008.

## Walk 2026-11-07 (before review remediation)

Build: `feat/file-explorer-s10` (integration `410a3566e` + `c5d1d54a6`), Electron fixture from
`bun run --cwd desktop build:e2e-update`, driven by the desktop Playwright harness with every
`COMPOZY_*` variable unset:

```sh
bun x playwright test --config playwright.config.ts --grep 'E2E-001 \(#1167\)|E2E-008: Open in'
```

| Step | Result | Evidence |
| --- | --- | --- |
| 1–3 Choose application → Cursor, remembered | pass | E2E-008: the menu lists Finder; choosing Cursor records `-g <project>/fixture.txt:7:1` and the primary reads "Open in Cursor" |
| 4 Modifier click uses the remembered target | pass | E2E-008: Cmd-click records the same Cursor argv; E2E-001 (#1167): with no remembered target, Cmd-click reaches the configured editor with zero dialogs, and the path is the "Open fixture.txt in viewer" button |
| 5 Finder reveal | not walked | `file-opener.test.ts` UT-062 covers reveal without spawning |
| 6 Confirmations | not walked | `file-opener.test.ts` UT-064 covers outside and project `.env` confirm/cancel |
| 7 Vanished target | not walked | UT-063 covers unknown and vanished targets |
| 8 Browser tab | pass | `ET-transcript-paths-files-app` step 5: no Open in or Choose application controls |

Observation: an inside credential file got the same sheet title, "Open file outside this
workspace?", as an outside path. Fixed in the review remediation (O-7): it now has its own title.
The walk steps above were rewritten to the spec behavior (O-1); this table predates them.

Blocker for the remaining steps: the packaged production app strips remote debugging, and the
instrumented E2E app records editors through fake executables, so real editor launches, real
Finder reveal and native sheet pixels need an operator walk on a packaged build with real
applications installed. Status stays `partial` until then.
