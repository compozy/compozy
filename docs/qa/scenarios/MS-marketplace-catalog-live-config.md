---
id: MS-marketplace-catalog-live-config
area: MS
title: Apply curated marketplace feed configuration live
persona: Vera
journey: J-extension-policy-admin
expected: Valid marketplace.catalog base_url, ttl, and timeout values load and apply live from global config; workspace overlays and workspace-scoped writes are rejected before persistence; invalid URLs or non-positive durations are rejected; a live global apply changes the source used by the next catalog refresh without restarting the daemon.
entry_points: global config.toml; agh config set marketplace.catalog.* --scope global; agh__config_set scope=global; rejected workspace config.toml/write attempts; marketplace.catalog.refresh event summaries; agh.network/runtime/core/configuration (config docs)
qa_status: untested
bug_ids: BUG-20260715-marketplace-config-set-live; BUG-20260715-marketplace-native-config-policy; BUG-20260715-config-set-late-metadata
fix_status: BUG-20260715-marketplace-config-set-live fixed; BUG-20260715-marketplace-native-config-policy fixed; BUG-20260715-config-set-late-metadata fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-20260715-20260715-114240-757254-lab/qa-artifacts/qa/notes/marketplace-config-set-live.json; /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-config-reachability.json
last_report: docs/qa/reports/2026-07-15-marketplace.md
overlaps: MS-033; ET-marketplace-kill-switch
---

The next settings-focused QA cycle should use two isolated local feed servers to prove that a live
`base_url` change affects the next refresh, then submit invalid URL, zero TTL, and negative timeout
values through the public configuration surfaces. Confirm the daemon preserves the prior active
configuration after a failed apply and records refresh outcomes without exposing feed response data.

2026-07-15 final retest: the public CLI and native tool preserve valid values after invalid URL and
duration attempts. A live two-feed switch advanced the active generation, replaced two primary MCP
rows with the single secondary row, then restored the primary feed without a restart. The replay
also closed the slow-boot metadata path that previously reported local writes as applied without a
daemon apply record.

QA impact 2026-07-16: catalog config is now explicitly global-only. Add manual overlay, CLI, and
native workspace rejection checks; historical global live-apply evidence remains relevant. Native
global TTL/timeout set and unset must expose the settings apply record, advance the active generation
on success, and return `applied=false`, `next_action=retry`, and reconciliation diagnostics when the
runtime apply fails.
