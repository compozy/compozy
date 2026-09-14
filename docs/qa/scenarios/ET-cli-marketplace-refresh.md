---
id: ET-cli-marketplace-refresh
area: ET
title: Refresh curated marketplace catalogs from the CLI
persona: Ada
journey: J-agent-marketplace-parity
expected: "Refresh returns one outcome per enabled source; exit 1 only when all source refreshes fail."
entry_points: compozy marketplace refresh -o json; POST /api/marketplace/refresh over HTTP and UDS
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-07-30-mcp-2026-catalog-v2.md;/Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-08-02-bundles-removal.md
overlaps: MS-marketplace-catalog-live-config; ET-marketplace-kill-switch
---

Reset for the source-based catalog hard cut. Historical evidence above covers the retired contract. Use an isolated feed plus two plugin sources; fail one, then all, and verify every outcome and recovery without losing installed extensions. No --kind selector remains. Final walk: task_10.
