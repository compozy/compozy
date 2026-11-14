## 0.7.1 - 2026-11-14

### Bug Fixes

- Re-check a stale needs_login provider after a verified usage collection (#1697)
- Cap reasoning effort levels to the model profile (#1704)
- Stop reporting a deleted child session as working (#1693)
- Keep the Archived and Later session list views when opening a session (#1707)
- Show the session's agent avatar on session tabs (#1710)
- Stop E2E leak checks from matching Vite asset hashes (#1714)
- List unfoldered sessions beside folders in Archived and Later (#1718)

### Performance Improvements

- Remove idle shell subscriptions from tab switching (#1701)

### Testing

- Assert Markdown Preview default in Files E2E-002 (#1699)
