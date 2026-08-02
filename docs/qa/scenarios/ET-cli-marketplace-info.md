---
id: ET-cli-marketplace-info
area: ET
title: Resolve marketplace detail by stable entry identity
persona: Ada
journey: J-agent-marketplace-parity
expected: `compozy marketplace info <kind> <entry_id>` returns the same typed detail as HTTP and UDS and reports deterministic 400 or 404 errors for invalid identity.
entry_points: compozy marketplace info <kind> <entry_id> -o json; GET /api/marketplace/:kind/:entry_id over HTTP and UDS
qa_status: untested
bug_ids: BUG-20260729-marketplace-json-parity
fix_status: fixed
retest_status: pass
fix_commits: 351f3535
evidence: docs/qa/reports/2026-07-30-mcp-2026-catalog-v2.md
last_report: docs/qa/reports/2026-07-30-mcp-2026-catalog-v2.md
overlaps: ET-008
---

Planning note: installed-name collision disambiguation remains pending; no bug fix is associated with this scenario.

Added by marketplace Task 02. QA should use an entry whose display name differs from its entry_id so resolution cannot accidentally regress to display-name lookup.

QA impact 2026-07-18: `--installed-name` now selects the exact installed MCP, extension, or skill
when its local identity collides with a curated `entry_id`; compare CLI, HTTP, and UDS detail.

QA impact 2026-08-02: detail accepts exactly MCP, extension, and skill kinds. Reset to verify exact
installed identity and deterministic rejection of unsupported kinds.

QA result 2026-07-29: CLI JSON added `resolution_source` outside the shared Marketplace detail
payload. The canonical regression, staged root fix, and rebuilt CLI/HTTP/UDS replay are green; the
scenario remains failed until the fix has a governed commit.
