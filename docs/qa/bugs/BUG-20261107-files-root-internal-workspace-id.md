# BUG-20261107-files-root-internal-workspace-id: Open in Files window opens "Project not found"

- Status: verified (fixed on feat/file-explorer-s10, commit c5d1d54a6)
- Impact (user-side): Broken action
- Severity: Medium; Priority: P2
- Persona Affected: Dora
- Journey Step: Review panel Files tab → Open in Files window
- Scenarios: ET-session-review-files, ET-files-app
- Origin: file-explorer S10 integrated QA walk
- Report: docs/qa/reports/2026-11-07-file-explorer-integration.md

## Reproduction

In a project session, open the Review panel on Files and choose **Open in Files window** from the
tree header. A Files window opens keyed by an ID such as `01M6TF722A9173YEW7QCDG2FDJ` and reads
"Project not found · It was removed from CompozyOS." The dock's Files window for the same project is
keyed `ws_…`, so the two never match.

Root cause: every file, search and changes response sets `root.workspace_id` to the project's
internal identity (`ResolvedWorkspace.WorkspaceID`) instead of the public registry ID that routes,
sessions and the window manager use. The same value reached `compozy session changes -o json` and
`compozy__session_changes`. The web passes it to `requestFilesAppPlace`/`openFiles`.

## Fix and re-walk

`projectFileRoot` (HTTP/UDS core) and the native `compozy__session_changes` binding now set the
root's workspace ID to the public registry ID. Regressions: `TestProjectFileContentReveal` (core
fixture with distinct registry and identity IDs) and `TestNativeSessionChanges` (fixture
`nativeTestWorkspaceServiceWithRootAndIdentity`); both fail without the fix. The real-daemon
`TestDaemonSearchSessionFilesWorktreeBoundIT005` passes. Re-walk on the fixed daemon: the window opens
keyed `ws_e72b1865b5dccbd5` with `README.md` open.
