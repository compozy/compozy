---
id: ET-api-marketplace-namespace
area: ET
title: Operate the unified marketplace API namespace
persona: Ada
journey: J-agent-marketplace-parity
expected: Search, kind browse, stable entry detail, and refresh expose matching HTTP and UDS contracts; extension detail includes its exact HTTPS artifact URL; native discovery uses the caller's exact workspace installed-state projection; every deleted skills and extensions browse route returns 404.
entry_points: GET /api/marketplace/search; GET /api/marketplace/:kind; GET /api/marketplace/:kind/:entry_id; POST /api/marketplace/refresh; compozy__marketplace_search
qa_status: untested
bug_ids: BUG-20260715-native-marketplace-extension-parity; BUG-20260715-marketplace-stale-report; BUG-20260729-marketplace-file-cursor-fence
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38;351f3535
evidence: /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-agent-parity-final.json; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/022-marketplace-namespace
last_report: docs/qa/reports/2026-07-30-mcp-2026-catalog-v2.md
overlaps: ET-007; ET-008; ET-016
---

Skipped in the 2026-07-30 MCP 2026/catalog-v2 closeout: no equivalent HTTP and UDS API read was retained.

Historical QA note: authored parameter, enum, and status contract verification remains pending.

Added by marketplace Task 02. QA should include empty-query idle slices, per-kind error isolation, global and workspace installed-state projection, non-loopback HTTP refresh denial, unguarded UDS refresh, and 404 checks for the hard-deleted legacy routes.

QA impact 2026-07-16: the authored OpenAPI now owns each Marketplace operation's exact parameter
list, scope/kind enums, and success/error statuses; compare generated clients and docs with runtime.

QA impact 2026-08-02: call `compozy__marketplace_search` from two workspaces with homonymous MCP and
extension installs and prove each result projects only the caller's installed state.

QA impact 2026-07-16: inspect the default Repository Orientation extension detail over HTTP and UDS;
both must expose the same `artifact_url`, digest, install slug, and repository metadata.

QA impact 2026-07-18: single-kind browse now returns opaque `next_cursor` continuation and exact
filtered totals where the source can determine them. Prove stable, non-overlapping pages and reject
a cursor replayed with a different kind, query, scope, or workspace.

QA impact 2026-07-18: grouped search no longer advertises an unusable continuation cursor. A
single-kind cursor now carries a projection fence; mutate the curated catalog or remote-skill prefix
between pages and prove the daemon rejects the stale cursor with restart
guidance instead of returning duplicate or skipped entries.

QA impact 2026-07-27: under `make dev`, browse the checkout catalog through HTTP, UDS, CLI, and
`compozy__marketplace_search`; prove each surface returns the same Repository Orientation entry,
reports source unavailability as 503, ignores any fresh remote projection while the local source is
active, and leaves the production `main` default unchanged. Validate the published artifact digest
and current manifest field before install. Status remains untested; no QA session ran.

QA result 2026-07-29: an unchanged file-catalog refetch invalidated the first page's continuation
cursor because freshness was treated as catalog revision. The canonical regression, staged root
fix, and rebuilt cross-plane replay are green; the scenario remains failed until the fix has a
governed commit.
