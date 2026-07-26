---
id: ET-cli-marketplace-info
area: ET
title: Resolve marketplace detail by stable entry identity
persona: Ada
journey: J-agent-marketplace-parity
expected: `agh marketplace info <kind> <entry_id>` returns the same typed detail as HTTP and UDS and reports deterministic 400 or 404 errors for invalid identity.
entry_points: agh marketplace info <kind> <entry_id> -o json; GET /api/marketplace/:kind/:entry_id over HTTP and UDS
qa_status: untested
bug_ids:
fix_status:
retest_status: pending installed-name collision disambiguation
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-agent-parity-final.json
last_report: docs/qa/reports/2026-07-15-marketplace.md
overlaps: ET-008
---

Added by marketplace Task 02. QA should use an entry whose display name differs from its entry_id so resolution cannot accidentally regress to display-name lookup.

QA impact 2026-07-18: `--installed-name` now selects the exact installed MCP, extension, or skill
when its local identity collides with a curated `entry_id`; compare CLI, HTTP, and UDS detail.

QA impact 2026-07-18: `--installed-name` is rejected locally for bundle detail and is never sent by
the Web adapter for bundles. MCP, extension, and skill detail continue to send the exact identity.
