---
id: MS-live-model-release-refresh
area: MS
title: Discover a newly advertised model without a code update
persona: Ada
journey: J-20
expected: A model newly advertised by ACP, Cursor command discovery, configured discovery, or an extension source appears after TTL, periodic, or explicit refresh without replacing stale rows on failure; view=all exposes it even when explicit curation excludes it from the default view. A Claude release advertised only as a version alias (opus = "Opus 5.5") appears in the default composer list as its exact release id, one rejected per-model option probe does not mark the provider stale, and the composer picker refresh button re-runs discovery and either updates the list or shows the refresh error.
entry_points: compozy provider models list --all; compozy provider models refresh; HTTP/UDS model-catalog routes; compozy__provider_models_list|refresh; web composer model picker refresh
qa_status: pass
bug_ids: BUG-20260827-live-uncurated-model-admission
fix_status: fixed-pending-commit
retest_status: pending
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

Retest 2026-10-06 (desktop v0.3.0 report): the composer listed only the seeded Claude models while
claude-agent-acp advertised Opus 5.5, Sonnet 5.5, and Fable 5.1 as the `opus`, `sonnet`, and
`fable` aliases; the Claude live source flapped to `failed` whenever one per-model option probe was
rejected or exceeded the 10-second budget, which raised "Model catalog may be out of date"; and the
picker refresh button sent an unforced refresh that returned the cached failed status within its
TTL. Walk: open the composer picker with Claude Code installed and confirm the browse list shows
`claude-opus-5-5` "Opus 5.5" and `claude-sonnet-5-5` "Sonnet 5.5" first; click refresh and confirm
`POST /api/model-catalog/models/refresh` carries `"force": true` and the list rereads; with a
provider-rejected model in the Claude option list, confirm `compozy provider models status claude`
reports `provider_live:claude` as `succeeded` while that model shows unknown reasoning (no seeded
effort levels) with a "model options unavailable" error; when a forced composer refresh returns 200
but a signed-in provider's live source failed, the picker shows "Couldn't refresh <provider> models:
<error>".
