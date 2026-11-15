## 0.7.2 - 2026-11-15

### Bug Fixes

- Reuse profile scope without loop stream lookups (#1740)
- Retain managed subprocess stdout after exit (#1748)
- Preserve session authority and accepted lifecycle changes (#1745)
- Preserve long Loop gate leases and clean owned processes (#1746)
- Settle deleted subagent children and deliver parent wakes (#1744)
- Make the composer MCP control an icon-only neutral last control (#1757)
- Open Settings when runtime MCP servers have no env or headers (#1756)
- Authenticate daemon GitHub requests and honor rate limits (#1751)

### Performance Improvements

- Stop per-string secret encoding and per-session MCP discovery (#1755)

### Release Notes

#### Fixes

##### Reuse existing GitHub credentials and honor quota deadlines

GitHub update checks, marketplace sources and extension discovery use `GITHUB_TOKEN` from the
daemon environment, then the daemon user's `gh auth token` login for `https://api.github.com`,
then anonymous access. Custom API origins never receive the CLI's github.com credential.
Credentials require HTTPS, with existing local HTTP registries preserved on `127.0.0.1`, `::1`,
or `localhost` at the exact configured origin and port, including publishing. Other HTTP GitHub
API endpoints fail before sending a request and explain how to switch to HTTPS.

Quota failures honor GitHub's retry deadlines, preserve the last good plugin catalog, and explain
how to authenticate or retry. A new login takes effect after the retry deadline or a daemon restart.

##### Reuse the current profile when opening Loop streams

Loop run streams reuse the active or window-pinned profile without starting extra profile lookups when they open or close.

##### Preserve session authority and finish accepted cleanup and layout changes

Legacy sessions belonging to delegated always-on instances again retain their own tool and MCP
server restrictions after upgrading. Corrected catalog permission intent also applies when a
development build wrote stale intent markers into a session's metadata.

Deleting stopped history whose metadata is missing now removes its event database and attachments,
with the same owner checks and recoverable staging as normal deletion. Subagent cancellation finishes
its accepted settlement after a client disconnects. Failed roster removal leaves a terminal row that
can be deleted again even after its removal deadline expires. Archived cleanup bounds individual
deletions, including lifecycle-lock admission, and stops cached child
activity atomically. Raw session streams end on a replayed stop and fall back to polling after a failed
gap read.

Accepted layout replacements, migration actions, layout profile writes and client changes now survive
client disconnection within a bounded lifetime. Workspace checks and revision/version conflicts still
apply. Automation detail stays in its loading state while its window acquires live data, subagent
pages avoid duplicate rows, and removing an inspected subagent elsewhere closes its drawer after
confirmation for the current selection.
`compozy daemon stop` recognizes recycled process IDs. Automation waits and batch worktree removal
honor the configured client request budget.

### Migration

Migration 00204 appends a permission-intent correction; shipped 00202 and 00203 remain unchanged.
Stored lists and ancestor ceilings are retained. The original second-resolution 00202 cutoff is
preserved: rows in its ambiguous fractional second are not widened automatically. An affected user
can explicitly re-grant tool or MCP access to that session. Malformed historical permission JSON still
requires diagnosis rather than silently skipping shipped integrity failures.
