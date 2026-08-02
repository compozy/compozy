---
id: ET-cli-marketplace-refresh
area: ET
title: Refresh curated marketplace catalogs from the CLI
persona: Ada
journey: J-agent-marketplace-parity
expected: `compozy marketplace refresh` returns structured per-kind outcomes and accepts exactly the feed-backed `mcp`, `extension`, and `skill` values.
entry_points: compozy marketplace refresh -o json; compozy marketplace refresh --kind <mcp|extension|skill> -o json; POST /api/marketplace/refresh over UDS
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/reports/2026-07-30-mcp-2026-catalog-v2.md;/Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-08-02-bundles-removal.md
overlaps: MS-marketplace-catalog-live-config; ET-marketplace-kill-switch
---

Added by marketplace Task 02. QA should exercise success and stale/failure outcomes through an
isolated local feed and confirm unsupported kinds are rejected without mutating projection state.

QA impact 2026-08-02: the Marketplace kind set is now exactly MCP, extension, and skill. Reset for
the next QA cycle.
