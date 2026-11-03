---
id: APP-open-transcript-paths-in-editor
area: APP
title: Open file paths from an assistant message in the editor
persona: Dora
journey: J-desktop-attach-daily
expected: In the packaged desktop app, Command or Control click (or the hover Open in editor button) on a path in an assistant message opens that file at its line in the configured editor; outside-workspace and credential paths ask first and Cancel opens nothing; failures toast; browser tabs on the same daemon render exactly as before; the Settings Editor command applies without restart.
entry_points: packaged desktop main window transcript (inline code and markdown links in assistant messages); Settings › General › Editor command; compozy config set desktop.editor_command
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: lab compozy-open-paths-in-editor-20261103-210912-165108 (qa-artifacts/qa/walk-results.json, journey-log.jsonl, screenshots/); key PNGs and argv recordings in /tmp/opie-qa-evidence/
last_report:
overlaps: APP-desktop-native-edit-shortcuts
---

Introduced for issue #1167 (spec `open-paths-in-editor`, US-001–US-007). Walk it once on the
integrated branch with a real packaged desktop app and a real editor (for example VS Code with
`code -g {path}:{line}:{col}`, or `zed {path}:{line}`).

Walk:

1. In Settings › General › Editor command, set the editor template and save. Confirm the save
   applies without a restart prompt and that `compozy config get desktop.editor_command -o json`
   returns the same value. Enter `code -g` (no `{path}`) and confirm the inline error names
   `desktop.editor_command`.
2. In a project session, have the agent reply with `` `internal/<real file>.go:42` ``. Hover the
   span: a dotted underline and the Open in editor button appear; the tooltip reads "Open in
   editor" with ⌘-click (macOS) or Ctrl-click. Plain click selects text only. Command or Control
   click opens the file at line 42 with no dialog. Tab to the button and press Enter: same result.
3. Ask for `` `~/.ssh/config` ``. Command or Control click shows the native "Open file outside this
   workspace?" dialog with the full path and "It is outside the session's workspace." (`_uiux.md`
   S3 reserves "It may contain credentials." for a sensitive target inside a root, EC-5); Cancel
   opens nothing and no toast appears.
4. In a User-scope session, mention `` `~/notes.md` ``. The click asks first (home is never an
   inside root); Open opens it.
5. Mention a missing path (`` `src/missing-file.ts` ``): the toast reads "File not found: <path>".
   Mention a directory with an extension-like name: the toast reads "Not a file: <path>".
6. Have the agent write `[cfg](~/.compozy/config.toml)`: the link text reads as a link, plain click
   does nothing and never navigates, Command or Control click opens after the confirmation.
7. Set the editor command to a missing executable (`nope-editor {path}`): the click toasts
   "Couldn't open the editor. Check the editor command in Settings." with Open Settings, which
   lands on Settings › General. Restore the working command and click again without restarting.
8. Open the same session in a browser tab on the same daemon: no underline, no button, inline code
   unchanged, and `[cfg](~/.compozy/config.toml)` still renders as `cfg [blocked]`.

Owning automated coverage: web `file-path-links.test.ts` (UT-030/UT-031), `message-markdown.test.tsx`
(UT-040–UT-046, UT-048), `session-messages.test.tsx` (UT-044 reply card), Settings
`-general.test.tsx` and `use-settings-general-page.test.tsx` (UT-047), desktop opener suites
(UT-010–UT-021), and the packaged shell E2E-001 in `desktop/e2e/_electron/__tests__/shell.spec.ts`.

## Walk 2026-11-03

Build: integration head `cbae83249` (branch `feat/open-paths-in-editor-qa`), the instrumented
packaged macOS arm64 app from `bun run --cwd desktop build:e2e-update`
(`desktop/.artifacts/e2e-baseline/mac-arm64/CompozyOS.app`, bundled runtime from the same head) with
`web/dist` from `bun run --cwd web build`. The production package strips remote debugging, so the
walk drove the E2E-instrumented package through Playwright Electron. Isolated targeted lab
(`eng-qa-bootstrap`, own `COMPOZY_HOME`, port and socket), the desktop app owning the lab daemon, the
acpmock provider, and a fake `HOME` holding fake `.ssh/config`, `.compozy/config.toml` and
`notes.md`. The editor was a recorder script logging argv (`"node" record-A.mjs {path} {line}`), not
a real editor. Native dialogs were recorded at the `dialog.showMessageBox` boundary: the real sheet
opened and closed through its `signal`, and Cancel kept the real response.

| Step                              | Result | Evidence                                                                                                                                                                                                              |
| --------------------------------- | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1 Settings editor command         | pass   | Save applied without restart (same shell pid); `config get` returned the saved template; `code -g` showed "settings validation error: desktop.editor_command must contain {path}" on the row (`J7-settings-saved.png`, `J7-settings-invalid.png`) |
| 2 Inside path `fixture.txt:7`     | pass   | Dotted underline and button on hover, tooltip "Open in editor ⌘-click"; plain click recorded nothing; ⌘-click and Tab-to-button + Enter each recorded `[<project>/fixture.txt, "7"]` with zero dialogs (`J1-inside-hover.png`, `editor-argv.jsonl`) |
| 3 `~/.ssh/config` outside         | pass   | Dialog "Open file outside this workspace?", detail with the full fake-home path and the outside sentence, buttons Open/Cancel; Cancel opened nothing and showed no toast (`J2-ssh-after-cancel.png`, `walk-results.json`) |
| 4 User-scope `~/notes.md`         | pass   | Dialog shown (home root discarded); Open recorded `[<fake-home>/notes.md, "1"]`                                                                                                                                       |
| 5 Missing path and directory      | pass   | Toasts "File not found: <project>/src/missing-file.ts" and "Not a file: <project>/src/fixture-dir.ts" (`J4-missing-toast.png`)                                                                                         |
| 6 `[cfg](~/.compozy/config.toml)` | pass   | Rendered as link text; plain click did not navigate, record or prompt; ⌘-click showed the dialog and Open recorded `[<fake-home>/.compozy/config.toml, "1"]`                                                          |
| 7 Missing executable              | pass   | `nope-editor {path}` toasted "Couldn't open the editor. Check the editor command in Settings." with Open Settings, which landed on `/settings/general`; restoring the recorder applied on the next click without restart (`J7-editor-failed-toast.png`) |
| 8 Browser tab                     | pass   | Chromium tab on the lab daemon: zero `[data-file-path]` spans, zero open buttons, ⌘-click recorded nothing, the link rendered "Edit cfg [blocked] to change it." (`J6-browser-tab.png`)                               |

Not walked: a real editor, and EC-5 (workspace rooted at `~/.ssh`, which should show "It may
contain credentials."). The EC-5 attempt did not reach a verdict because the transcript span stayed
hidden in that new workspace view. The desktop opener suites cover EC-5. Native-sheet pixels were
not kept: the region capture caught other desktop windows, so the recorded `showMessageBox` options
are the dialog evidence.
