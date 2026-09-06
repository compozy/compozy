# BUG-20260906-injected-guidance-missing-history: Delivered steering disappears from conversation history

- **Status:** fixed — real-provider re-walk and reload verified
- **Impact (user-side):** Data-Loss
- **Severity:** High · **Priority:** P0
- **Persona Affected:** Théo
- **Journey Step:** J-13 follow a live run, steer and later revisit the instruction
- **Scenarios:** RT-session-live-steer; ET-web-session-transcript-calm-grammar
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

## Summary

The operator receives a successful injected steering receipt and Claude applies the instruction, but the authored guidance is absent from the persisted conversation, search and message trail. The delivery marker remains without the message it describes.

## Reproduction

- **Charter:** CH-managed-session-intervention · **Tour:** Feature Tour
- **Environment:** isolated current-build daemon, CLI + public HTTP, real Claude Code / claude-sonnet-5.

1. Create a session and request twelve release-note drafts, one file per area.
2. During turn `turn-4f009525b74d74a6`, send `For every draft, use the exact heading Operator workflow before the examples. Keep the existing twelve-file plan.` with message_id `msg_release_guidance_01` and idempotency_key `idem_release_guidance_01`.
3. Observe `disposition=steering`, `steer_delivery=injected`, same turn, entry `inq-77a267cae0bb2c6b`.
4. Read `session history`, `session search 'For every draft'`, `session outline` and the public HTTP transcript after the turn settles.

**Expected:** one durable authored guidance message, original identity and same-turn delivery provenance, discoverable through search and outline.
**Actual:** the text and identity are absent, search returns no matches and outline contains only the initial prompt, interrupt replacement and queued follow-ups.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-sessions-stability-integrated-20260906-102255-367122-lab/qa-artifacts/qa/evidence/release-guidance.json`
- `/Users/pedronauck/dev/qa-labs/compozy-sessions-stability-integrated-20260906-102255-367122-lab/qa-artifacts/qa/evidence/release-history-live.json`
- `/Users/pedronauck/dev/qa-labs/compozy-sessions-stability-integrated-20260906-102255-367122-lab/qa-artifacts/qa/evidence/release-guidance-search.json`
- `/Users/pedronauck/dev/qa-labs/compozy-sessions-stability-integrated-20260906-102255-367122-lab/qa-artifacts/qa/evidence/release-outline.json`
- `/Users/pedronauck/dev/qa-labs/compozy-sessions-stability-integrated-20260906-102255-367122-lab/qa-artifacts/qa/evidence/release-transcript-cold.json` — complete 134-sequence read, has_older=false.

## Fix

- **Root cause:** the injection path settles its durable queue entry and emits a delivery marker without recording the authored user event; normal queued dispatch owns that write only for replacement prompts.
- **Fix commit:** pending delivery commit.
- **Regression test:** existing `TestManagerLiveSteerDelivery` owns exact authored identity and confirmed asynchronous delivery; fallback must still produce exactly one correctly attributed user message.

## Verification

Fresh real Claude / sonnet session sess-35ceb519e6005e15 injected msg_handbook_guidance_02 into turn-f3d48e7247b52982. After settling and two daemon upgrades/restarts, the public transcript retains exactly one authored guide at sequence71, search finds its user text, outline retains the same turn/preview, and the real Web bubble shows Steered — delivered into the live turn. Evidence in the integrated lab: handbook-guidance.json, handbook-transcript-migrated.json, handbook-search-reloaded.json, handbook-outline-reloaded.json, handbook-web-reloaded.json and handbook-guidance-reloaded.png. Pending and superseded receipts retain authored text until confirmed delivery; final async/fallback canonical race checks are in .cache/sessions-qa-steer-final.log. Historical lost guide content predates the repair; no claim that missing pre-repair text was reconstructed.
