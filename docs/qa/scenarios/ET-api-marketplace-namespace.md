---
id: ET-api-marketplace-namespace
area: ET
title: Operate the unified marketplace API namespace
persona: Ada
journey: J-agent-marketplace-parity
expected: Search, kind browse, stable entry detail, and refresh expose matching HTTP and UDS contracts; extension detail includes its exact HTTPS artifact URL; native discovery uses the caller's exact workspace installed-state projection; every deleted skills and extensions browse route returns 404.
entry_points: GET /api/marketplace/search; GET /api/marketplace/:kind; GET /api/marketplace/:kind/:entry_id; POST /api/marketplace/refresh; agh__marketplace_search
qa_status: untested
bug_ids: BUG-20260715-native-marketplace-extension-parity
fix_status: fixed
retest_status: pending authored parameter, enum, and status contract verification
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-agent-parity-final.json
last_report: docs/qa/reports/2026-07-15-marketplace.md
overlaps: ET-007; ET-008; ET-016
---

Added by marketplace Task 02. QA should include empty-query idle slices, per-kind error isolation, global and workspace installed-state projection, non-loopback HTTP refresh denial, unguarded UDS refresh, and 404 checks for the hard-deleted legacy routes.

QA impact 2026-07-16: the authored OpenAPI now owns each Marketplace operation's exact parameter
list, scope/kind enums, and success/error statuses; compare generated clients and docs with runtime.

QA impact 2026-07-16: call `agh__marketplace_search` from two workspaces with homonymous bundle
activations and prove each result projects only the caller's activation and drift state.

QA impact 2026-07-16: inspect the default Repository Orientation extension detail over HTTP and UDS;
both must expose the same `artifact_url`, digest, install slug, and repository metadata.

QA impact 2026-07-18: single-kind browse now returns opaque `next_cursor` continuation and exact
filtered totals where the source can determine them. Prove stable, non-overlapping pages and reject
a cursor replayed with a different kind, query, scope, or workspace.

QA impact 2026-07-18: grouped search no longer advertises an unusable continuation cursor. A
single-kind cursor now carries a projection fence; mutate the curated catalog, remote-skill prefix,
or bundle projection between pages and prove the daemon rejects the stale cursor with restart
guidance instead of returning duplicate or skipped entries.
