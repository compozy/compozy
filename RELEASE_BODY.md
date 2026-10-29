## 0.4.5 - 2026-10-29

### Bug Fixes

- Preserve default profile customization across daemon startup (#1001)
- Preserve Linux update executor identity across clock drift (#1004)
- Restore terminal lifecycle under All projects and User (#1009)
- Preserve archive recovery and pause rejected automation replacements
- Unblock missing-root recovery and restore native input contracts
- Preserve picker focus and synchronize subagent navigation checks
- Preserve native tool domain errors and sanitized causes (#1014)
- Recover detached message edits after lost acknowledgment (#1018)
- Isolate missing workspace state from healthy operations (#1016)
- Register SQLite Unicode search before opening connections
- Make Goal controls interrupt work and clarify session lifecycle (#1019)
- Expose Compozy native tools in Pi sessions (#1021)

### Features

- Change existing session worktrees in place (#1002)
- Inspect subagents in a resizable activity drawer (#1007)
- Archive projects and add safe removal actions (#1010)

### Performance Improvements

- Accelerate session and app data loading (#1006)

### Testing

- Wait for subagent branch expansion before paging
- Account for uptime in denied settings mutation checks
