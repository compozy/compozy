# BUG-20260715-run-conversation-hardcoded-empty: Run detail discards durable coordination history

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-enable-coordinated-conversations, watch the future run conversation and usage
- **Scenarios:** NB-run-conversation-bounds-usage
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-14-network-changes.md

## Summary

The task-run Web route constructed a permanently empty conversation and zero usage locally. A durable message already existed in the run's `thread_agent_channel`, but run detail showed silence and gave operators no deterministic conversation reference, pagination, or live update path.

## Reproduction

- **Charter:** CH-coordination-future-runs · **Tour:** Back-Button Tour
- **Environment:** desktop / isolated local daemon / en-US

1. Start one future Live coordinated run and persist a message in its run channel.
2. Read the durable conversation through the Network store/API.
3. Open the public task-run Web route.

**Expected:** Run detail pages the durable history, exposes run-fenced bounds/usage, and receives later messages over SSE.
**Actual:** The route always rendered an empty array, zero messages, and zero usage regardless of persisted rows.

## Evidence

- `docs/qa/evidence/2026-07-14-network-changes/ch-coordination-future-runs.md`
- Live retest: `msg-ch3-live-001` appeared without reload; the first 120-message page loaded five older rows without duplicates; refresh restored the page and pagination state.

## Fix

- **Root cause:** the public task-run detail contract had no run Network projection, and the Web route populated placeholder conversation/usage values instead of consuming the durable store.
- **Fix commit:** pending final whole-diff commit.
- **Regression tests:** the canonical GlobalDB usage-budget suite, API run-detail/SSE suite, generated contract checks, route integration, conversation hook, and panel suite own run fencing, paging, live invalidation, empty/error states, bounds, and actual-or-unavailable usage.

## Verification

- **Retested:** 2026-07-15, same persona/journey · **Report:** docs/qa/reports/2026-07-14-network-changes.md
- **Result:** public run detail exposes a deterministic workspace/channel/thread reference, paginated history, typed `network.message`/`network.usage` SSE, and run-bound consumption without granting conversation text Task authority.
