## 0.7.5 - 2026-11-18

### Bug Fixes

- Make maestro dispatch monitors mandatory and checkable
- Wake always-on mains when provenance children settle (#1786)
- Retain MCP tools through refreshable OAuth expiry (#1789)
- Archive executor children during post-merge cleanup
- Resolve supported defaults for Cursor reasoning (#1793)
- Archive every session bound to a merged worktree during cleanup
- Prevent session window crash when the same image is pasted twice (#1805)
- Normalize remote MCP tool names (#1806)
- Enable Agent Plugin MCP sign-in and secret bindings (#1810)
- Leave an emptied desktop without tabs instead of seeding a New Tab (#1804)
- Restore daemon boot and session authority reads (#1811)

### Performance Improvements

- Stop shell-wide re-renders that pushed tab switches over budget (#1792)
- Reduce steady-state daemon policy and MCP CPU (#1802)

### Testing

- Enforce deterministic tests with landmines lint rules (#1808)
