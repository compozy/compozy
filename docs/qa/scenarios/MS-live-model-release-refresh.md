---
id: MS-live-model-release-refresh
area: MS
title: Discover a newly advertised model without a code update
persona: Ada
journey: J-20
expected: A model newly advertised by ACP, Cursor command discovery, configured discovery, or an extension source appears after TTL, periodic, or explicit refresh without replacing stale rows on failure; view=all exposes it even when explicit curation excludes it from the default view.
entry_points: compozy provider models list --all; compozy provider models refresh; HTTP/UDS model-catalog routes; compozy__provider_models_list|refresh
qa_status: pass
bug_ids: BUG-20260827-live-uncurated-model-admission
fix_status: fixed-pending-commit
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/live-model-refresh-cold-open.json
last_report: docs/qa/reports/2026-08-27-acp-runtime-catalog.md
overlaps: RT-model-catalog-cold-open; MS-042
---

Added for the ACP runtime catalog rebuild. This scenario owns the root-cause promise that provider
releases do not require a CompozyOS transport switch or seed update. Explicit curation still owns
default-view membership.

The dynamic-source scheduler covers provider-live and extension sources. Both expose the same
five-minute TTL and retain their last successful rows when a background, read-triggered, or explicit
refresh fails.

QA 2026-08-27: an isolated Cursor discovery source advertised the synthetic logical model
`qa-future-1`, which had no seed or source-code entry. An explicit refresh published it consistently
through CLI, HTTP, and the native tool. A same-source failure retained it as `available_stale`; a
daemon restart returned the persisted row in 0.02 seconds; restoring real discovery removed the
synthetic row and returned 33 live account models including `grok-4.6`.

The runtime admission regression found during the same walk is fixed: explicit model validation now
uses the complete live view, while curation controls only the default browsing view. Focused Manager
coverage proves the view choice; the automatic release and stale-retention walk remains pass.
