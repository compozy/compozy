---
id: APP-open-in-targets
area: APP
title: Open a viewed file in a detected desktop application and remember the target
persona: Dora
journey: J-desktop-attach-daily
expected: In the packaged desktop app, the viewer header's Open in split button lists the Configured editor, every detected editor (login-shell PATH or macOS bundle) and Finder/File manager; choosing one launches it at the file and line without a shell and remembers it as the primary action; Cmd/Ctrl-click on a transcript path opens that remembered target directly; outside-project and credential files ask first and Cancel launches nothing; Finder only reveals; browser tabs show no Open in control.
entry_points: packaged desktop main window: Review panel and Files app viewer header (Open in split button, Choose application); viewer "Outside this project" state; transcript path Cmd/Ctrl-click; Settings › General › Editor command
qa_status: partial
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: desktop/e2e/_electron/__tests__/shell.spec.ts E2E-008 and E2E-001 (#1167) on the S10 head (/tmp/s10/desktop-e2e.log, 2 passed in 20.0 s)
last_report: docs/qa/reports/2026-11-07-file-explorer-integration.md
overlaps: APP-open-transcript-paths-in-editor; ET-session-review-files
---

Introduced by the file-explorer spec (US-006, US-011, US-005.AC-4; ADR-003; `_uiux.md` S5). It
replaces the "Cmd-click opens the configured editor only" behavior of `APP-open-transcript-paths-in-editor`.
Walk it once on a packaged desktop app with at least two real editors installed (for example VS Code
and Zed) and `desktop.editor_command` set.

Walk:

1. Open a file in the Review panel viewer. The header shows **Open in <target>** with a
   **Choose application** chevron. The first time, the primary target is **Configured editor**.
2. Open Choose application. The menu lists Configured editor, every detected application from the
   catalog (VS Code, Cursor, Zed, Sublime Text, IntelliJ IDEA, WebStorm, GoLand, Xcode), and Finder
   (macOS) or File manager. Applications that are not installed are absent; detection reads the
   login-shell PATH once per app run, plus macOS application bundles.
3. Choose an editor. It opens the file at the viewer's line (VS Code and Cursor get
   `-g <path>:<line>:<col>`; JetBrains IDEs and Xcode get `--line <line> <path>`). The primary button
   now reads **Open in <that editor>**, also after a reload (`compozy.open-target`).
4. In the transcript, Cmd-click (macOS) or Ctrl-click a path: it opens directly in the remembered
   target and does not route the Review panel. A plain click still opens the viewer.
5. Choose **Finder**: the file is revealed and selected, never opened or executed.
6. Open `.env` or a key file inside the project, or a path outside it, and choose a target: a native
   "Open file outside this workspace?" sheet shows the path and "It may contain credentials." (inside
   credential files) or "It is outside the session's workspace." Cancel launches nothing.
   `.env.example`, `.env.sample` and `.env.template` open without asking.
7. Make the remembered editor unavailable (uninstall it, or remove it from PATH and relaunch). The
   bridge refuses it (`target_unavailable`), nothing launches, and the menu no longer lists it.
8. In a browser tab on the same daemon, the viewer has no Open in control, and Cmd/Ctrl-click opens
   the viewer.

Owning automated coverage: desktop `file-opener.test.ts` (UT-060–UT-065), `product-contract.test.ts`,
`product-bridge-controller.test.ts` (main-frame authorization), `login-shell-path.test.ts`; web
`file-viewer-pane.test.tsx` (UT-055, target selection), `message-markdown.test.tsx` (UT-052); packaged
`shell.spec.ts` E2E-008.

## Walk 2026-11-07

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
| 8 Browser tab | pass | `ET-session-review-files` step 5: no Open in or Choose application controls |

Observation: an inside credential file gets the same sheet title, "Open file outside this
workspace?", as an outside path; only the detail line differs. The copy predates this spec (#1167).

Blocker for the remaining steps: the packaged production app strips remote debugging, and the
instrumented E2E app records editors through fake executables, so real editor launches, real
Finder reveal and native sheet pixels need an operator walk on a packaged build with real
applications installed. Status stays `partial` until then.
