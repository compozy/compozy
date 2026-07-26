---
id: ET-cli-marketplace-refresh
area: ET
title: Refresh curated marketplace catalogs from the CLI
persona: Ada
journey: J-agent-marketplace-parity
expected: `agh marketplace refresh` returns structured per-kind outcomes, accepts only feed-backed `--kind` values, and leaves derived bundle catalogs outside refresh.
entry_points: agh marketplace refresh -o json; agh marketplace refresh --kind <mcp|extension|skill> -o json; POST /api/marketplace/refresh over UDS
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-kill-switch.json; /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-agent-parity-final.json
last_report: docs/qa/reports/2026-07-15-marketplace.md
overlaps: MS-marketplace-catalog-live-config; ET-marketplace-kill-switch
---

Added by marketplace Task 02. QA should exercise success and stale/failure outcomes through an isolated local feed and confirm `--kind bundle` is rejected without mutating projection state.
