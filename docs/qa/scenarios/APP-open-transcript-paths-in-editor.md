---
id: APP-open-transcript-paths-in-editor
area: APP
title: Open file paths from an assistant message in the editor
persona: Dora
journey: J-desktop-attach-daily
expected: In the packaged desktop app, Command or Control click (or the hover Open in editor button) on a path in an assistant message opens that file at its line in the configured editor; outside-workspace and credential paths ask first and Cancel opens nothing; failures toast; browser tabs on the same daemon render exactly as before; the Settings Editor command applies without restart.
entry_points: packaged desktop main window transcript (inline code and markdown links in assistant messages); Settings › General › Editor command; compozy config set desktop.editor_command
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
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
   workspace?" dialog with the full path and the credentials sentence; Cancel opens nothing and no
   toast appears.
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
