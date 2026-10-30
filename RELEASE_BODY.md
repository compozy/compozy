## 0.4.6 - 2026-10-30

### Bug Fixes

- Make automatic titles lightweight and retry failed attempts
- Return conversation clear after durable reset
- Default terminals to the account login shell and open them from a launcher
- Refresh session rows in every catalog scope when the runtime changes
- Stop requests for unavailable desktop resources (#1038)
- Hand focus between project overlays before hiding their origins (#1040)
- Preserve Claude usage access across Keychain token refreshes (#1041)
- Restore project menubar focus after context menu dismissal
- Keep focused skip link out of desktop layout
- Report the enforced spawn budget to agents (#1048)
- Share terminal approvals through attention summary
- Restore workspace focus after worktree creation dialog

### Features

- Simplify the Fork session dialog and always fork into this window
- Selection-aware session context menu and project context menus (#1032)
- Group projects into folders and add session totals footer (#1047)
- Add proto.tsx prototype gallery that renders real web components (#1043)
- Open Kanban sessions in a resizable drawer over the board (#1055)
- Multi-select sessions and run bulk actions on the Kanban board (#1054)
- Inspect and control processes started by sessions (#1049)

### Testing

- Cover remaining merge train review invariants
- Synchronize detached harness session fixtures
- Record additive attention approval schema contract
