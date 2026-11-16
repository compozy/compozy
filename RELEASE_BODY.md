## 0.7.3 - 2026-11-16

### Bug Fixes

- Show reload failures on the closed composer MCP control (#1759)
- Exempt PR-covered issues from the intake watchdog
- Load trusted MCP sidecars before daemon validation (#1768)
- Pass CA trust variables to extension, MCP and hook subprocesses (#1764)
- Settle CompozyOS Loop joins behind an untaken gate route and explain contract failures (#1763)
- Escape only fence characters in check-in source items (#1767)
- Honor runtime providers for read-only triage (#1771)

### Performance Improvements

- Reduce retained-window and dock work on tab switches (#1742)
- Reduce hosted MCP projection and status read overhead (#1774)
