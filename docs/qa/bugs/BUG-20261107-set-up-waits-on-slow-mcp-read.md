# BUG-20261107-set-up-waits-on-slow-mcp-read: Set up stays disabled for up to 20 s after Add or page load

- Status: open
- Impact (user-side): Degraded (action unavailable while a spinner shows)
- Severity: Low; Priority: P3
- Persona Affected: Bruno
- Journey Step: Add a remote OAuth plugin, then press Set up
- Scenarios: ET-agent-plugin-marketplace-install
- Origin: spec marketplace-plugin-catalog S8 re-walk (isolated lab, 2026-11-07)

## Reproduction

Add `compozy-catalog/linear` (no sign-in). Reload `/marketplace?q=linear`. The Set up button renders
disabled with a spinner (`signInLoading` in `marketplace-catalog-row.tsx`) for 10–20 s, then shows the
warning icon and becomes clickable. The row waits on `GET /api/settings/mcp-servers/linear?scope=user&owner=extension:linear`,
which took 15.0 s and 9.2 s uncached and about 2 ms once cached.

Expected: Set up is usable right away. It can open detail with `?setup=1` while the live server read
is pending, and that read should not block on a remote probe.
