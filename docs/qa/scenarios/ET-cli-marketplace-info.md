---
id: ET-cli-marketplace-info
area: ET
title: Resolve marketplace detail by stable entry identity
persona: Ada
journey: J-agent-marketplace-parity
expected: `compozy marketplace info <kind> <entry_id>` returns the same typed detail as HTTP and UDS and reports deterministic 400 or 404 errors for invalid identity.
entry_points: compozy marketplace info <kind> <entry_id> -o json; GET /api/marketplace/:kind/:entry_id over HTTP and UDS
qa_status: pass
bug_ids: BUG-20260729-marketplace-json-parity
fix_status: fixed
retest_status: pass
fix_commits: 351f3535
evidence: /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-agent-parity-final.json; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/022-marketplace-namespace
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-008
---

Planning note: installed-name collision disambiguation remains pending; no bug fix is associated with this scenario.

Added by marketplace Task 02. QA should use an entry whose display name differs from its entry_id so resolution cannot accidentally regress to display-name lookup.

QA impact 2026-07-18: `--installed-name` now selects the exact installed MCP, extension, or skill
when its local identity collides with a curated `entry_id`; compare CLI, HTTP, and UDS detail.

QA impact 2026-07-18: `--installed-name` is rejected locally for bundle detail and is never sent by
the Web adapter for bundles. MCP, extension, and skill detail continue to send the exact identity.

QA result 2026-07-29: CLI JSON added `resolution_source` outside the shared Marketplace detail
payload. The canonical regression, staged root fix, and rebuilt CLI/HTTP/UDS replay are green; the
scenario remains failed until the fix has a governed commit.
