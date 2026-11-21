## 0.7.8 - 2026-11-21

### Bug Fixes

- Apply the quiet watchdog to silent ACP prompts (#1895)
- Keep session lists available when side ancestry is missing (#1901)
- Boot with unavailable workspace roots (#1905)
- Install the recorded app release while the channel feed is stale (#1904)
- Bound hosted MCP calls without caller deadlines (#1908)
- Document local sockets for stdio MCP servers (#1907)
- Accept Unicode separator schema digests (#1909)
- Exclude always-on mains from session group totals (#1911)
- Show Global always-on agents once, in the Global row (#1896)
- Show the inherited conversation in a derived child (#1910)
- Reject whitespace in pull request monitor branches (#1918)
- Keep children of deleted side chats hidden (#1919)
- Explain authorization-filtered peer lists (#1920)

### Performance Improvements

- Skip redundant metadata and SQL work in child-session supervision (#1893)
- Persist streamed transcript parts incrementally (#1912)
- Start desktop settings reads when the client registers (#1915)
- Bound page-cache traffic on Linux SQLite readers (#1914)
- Scope Linux busy-tick process scans (#1916)

### Refactoring

- Compose attention toasts in the components layer (#1921)

### Testing

- Leave the composer before E2E-016 presses the sidebar chord (#1894)
