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

Review regression checks: custom transport IDs stay available without being featured as Claude
release aliases. Cancelling an ACP inspection returns `context.Canceled` without publishing a
partial snapshot; exhausting only the probe deadline still returns the advertised models. The
existing `TestInspectSessionModels` subprocess suite covers cancellation during the first and
last probes, and `TestLiveProviderSources` covers custom-row ordering in the curated view.

QA impact 2026-10-18, opencode-model-discovery Task 03 (fixture proof only; live legs unverified): with the v1 fixture advertising `openrouter/acme/coding-default`, switch the fixture to v2 in place, press the composer picker refresh, and confirm `openai/qa-next` appears and is selectable without a code update. A malformed v2 payload after a valid row must fail the refresh while keeping the last complete rows stale; restoring a valid payload recovers on the next forced refresh. Changing the installation selector partitions rows under the new fingerprint; prior rows stay stored but no longer satisfy live/stale reads for the new invocation. Live vendor release/refresh legs remain unverified (Task 05 / V-001).

QA 2026-10-18, opencode-model-discovery Task 05 (live, installed v2.0.32): forced refresh publishes 92 live rows with no code update; 1s-timeout failure keeps prior rows stale and a forced retry recovers fresh; explicit-selector refresh partitions under the new fingerprint with the same row count. Report: [2026-10-18 task05](../reports/2026-10-18-opencode-model-discovery-task05.md). Live vendor release-delta (a genuinely new advertised model) and browser-picker refresh-button legs stay unverified.

QA impact 2026-10-18, selector follow-up: with an explicit curated set (including one consisting
entirely of deprecated entries), click the OpenCode chip without entering a search. Refresh under
v1, then switch the executable fixture to v2 and refresh again. Every advertised enabled model must
remain browseable and selectable with exact IDs; no curation rewrite or account substitution is
allowed. The All models curated projection and public catalog views remain unchanged.

Selector retest: **pass** in the daemon-served v1→v2 executable-fixture refresh journey, with
retired curation and empty-search selection (full provider-override spec 4/4). Live v2.0.32
picker replay also exposed all 92 available rows. This verifies the browsing regression, not a
genuinely new vendor release; see the selector follow-up in the
[2026-10-18 report](../reports/2026-10-18-opencode-model-discovery-task05.md).

QA regression 2026-10-19 (executable fixtures + SQLite): discover installation A successfully, switch
to B and refresh successfully, then return to A with version detection failing. A must return only
its own last complete live generation as `available_stale`, preserving its live-source refresh time
and last-success time; B must retain its fresh generation. The existing
`TestCatalogServiceGlobalDBIntegration/Should_transition_OpenCode_versions_and_isolate_invocation_partitions`
case verifies this sequence. No live-provider or browser re-walk is claimed.
