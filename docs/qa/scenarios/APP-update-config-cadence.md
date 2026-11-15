---
id: APP-update-config-cadence
area: APP
title: Control daemon update checks through the app config contract
persona: Dora
journey: J-desktop-agent-headless
expected: The daemon, not the shell, owns `[app].update_check` and `[app].update_check_interval`; disabling checks produces zero channel requests, changing the bounded interval affects the next daemon cadence without restart, and every config surface reads the same persisted global values.
entry_points: global config.toml [app]; compozy config get|set app.update_check and app.update_check_interval; config HTTP and UDS routes; compozy__config_get|set; configuration docs
qa_status: pass
bug_ids:
fix_status: 
retest_status: 
fix_commits: 
evidence: docs/qa/reports/2026-08-17-electron-shell.md
last_report: docs/qa/reports/2026-08-17-electron-shell.md
overlaps: APP-app-auto-update; APP-web-update-two-track
---

Added 2026-08-16 for the Electron cutover config lifecycle. The walk must prove that the shell does
not read or shadow either key, invalid intervals name the exact key, and sequential writes against
one isolated home converge across CLI, HTTP, UDS, native-tool, file, and live daemon read paths.


Update discovery regression (2026-10-18, issue #804):

- With no explicit interval, confirm automatic checks default to 15 minutes and discover a release
  while the same daemon remains running. With the idle 60-second UI poll, the normal visibility
  bound is about 16 minutes after publication, assuming successful upstream requests.
- Preserve explicit interval overrides and `update_check=false`. Manual checks stay available.
- In Settings, select **Check for updates** and confirm POST `/api/settings/update/check` reaches
  the daemon, returns the new shared snapshot, and updates `runtime.checked_at` only on success.
- Repeat the action from multiple clients within a minute: no additional upstream request. On
  upstream failure, retain the previous successful timestamp and show the failure. Honor GitHub
  `Retry-After` / `X-RateLimit-Reset` before retrying, including background checks.
- Reload Settings and confirm the same checked-at timestamp. An unchecked host must say
  **Not checked**, rather than **Up to date**.

GitHub authentication regression (2026-11-15, issue #1750):

- With `GITHUB_TOKEN` absent and `gh auth login` completed for the daemon user, check for an update
  and refresh a GitHub plugin source while the shared anonymous quota is exhausted. Both operations
  must use authenticated API requests without consuming anonymous quota. Do not record token values.
- Repeat without either credential. Keep anonymous access, cached catalog entries and the prior
  successful update timestamp. A 403 quota limit or 429 must show authentication state, the retry
  deadline and recovery guidance; repeated reads and forced refreshes must send no requests before
  the retry deadline (and primary reset deadline when its remaining quota is zero), and must
  recover after it.
- Verify redirects to another origin and HTTP destinations receive no Authorization header;
  diagnostics, events and provider environments contain no token. Missing or logged-out `gh` falls
  back silently; CLI lookup is bounded and its cache is memory-only.

- For a custom HTTPS GitHub API origin, leave `GITHUB_TOKEN` unset and verify that the
  github.com CLI login is not sent. An explicit environment token may authenticate that origin.
- Trigger a quota error during an individual plugin package download: keep the prior catalog,
  suppress explicit refreshes until the deadline, and recover after the quota resets.

Validation for this change runs in CI only. HTTP/TLS owning suites and the marketplace HTTP/SQLite
integration suite cover these boundaries; a live signed-in desktop walk remains unrun.
