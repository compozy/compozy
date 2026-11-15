---
id: ET-cli-marketplace-refresh
area: ET
title: Refresh curated marketplace catalogs from the CLI
persona: Ada
journey: J-agent-marketplace-parity
expected: "Refresh returns one outcome per enabled source; exit 1 only when all source refreshes fail."
entry_points: compozy marketplace refresh -o json; POST /api/marketplace/refresh over HTTP and UDS
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-07-30-mcp-2026-catalog-v2.md;/Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: MS-marketplace-catalog-live-config; ET-marketplace-kill-switch
---

Reset for the source-based catalog hard cut. Historical evidence above covers the retired contract. Use an isolated feed plus two plugin sources; fail one, then all, and verify every outcome and recovery without losing installed extensions. No --kind selector remains. Final walk: task_10.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

GitHub authentication regression (2026-11-15, issue #1750):

- With `GITHUB_TOKEN` absent and `gh auth login` completed for the daemon user, check for an update
  and refresh a GitHub plugin source while the shared anonymous quota is exhausted. Both operations
  must use authenticated API requests without consuming anonymous quota. Do not record token values.
- Repeat without either credential. Keep anonymous access, cached catalog entries and the prior
  successful update timestamp. A 403 quota limit or 429 must show authentication state, the retry
  deadline and recovery guidance; repeated reads and forced refreshes must send no requests before
  the retry deadline (and primary reset deadline when its remaining quota is zero), and must
  recover after it.
- Verify redirects to another origin receive no Authorization header;
  diagnostics, events and provider environments contain no token. Missing or logged-out `gh` falls
  back silently; CLI lookup is bounded and its cache is memory-only.

- For a loopback HTTP GitHub API origin (`127.0.0.1`, `::1`, or `localhost`), verify an explicit
  environment token reaches only the configured origin and port. HTTP endpoints outside those hosts
  must fail before network I/O with HTTPS recovery guidance, including hosts such as
  `localhost.example.test` and other loopback addresses.
- For a custom HTTPS GitHub API origin, leave `GITHUB_TOKEN` unset and verify that the
  github.com CLI login is not sent. An explicit environment token may authenticate that origin.
- Trigger a quota error during an individual plugin package download: keep the prior catalog,
  suppress later package requests to the same API origin during that refresh as well as explicit
  refreshes until the deadline, and recover after the quota resets. An independent source owner
  must retain its own quota state.

Validation for this change runs in CI only. HTTP/TLS owning suites and the marketplace HTTP/SQLite
integration suite cover these boundaries; a live signed-in desktop walk remains unrun.
