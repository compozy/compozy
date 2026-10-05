# BUG-20261005-loop-human-prompt-missing: A human gate asks for approval without its authored question

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Marina
- **Journey Step:** J-03, find and decide a waiting human gate
- **Scenarios:** LP-009; LP-010
- **Found:** 2026-10-05 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Marina sees a generic approval request instead of the author's question and artifact context. The durable approval event also reports Criteria=73 for one human criterion.

## Reproduction

CH-002, Interrupt Tour, 430x932 touch viewport at DPR 3, 4G, en-US.
Create studio-onboarding-review with the specific source-index review prompt. Open its parked run from Runs after the waiting event already exists. The card renders only the generic fallback question and description.

## Evidence

loops-marina-approval-ready.png and loops-marina-approve-events.json; the executed definition in loops-marina-after-offline-uds.json contains the authored prompt. Event seq 9 lacks prompt and says Criteria=73 despite one criterion.
All receipts are under docs/qa/evidence/2026-10-02-untested/. The exact twelve-frame
loops-review-marina recording is closed. No provider workers or mocked services are involved.

## Fix

The evaluator does not carry the rendered human prompt into durable diagnostics; the approval event drops it and counts the JSON byte length. The Web starts its stream after the timeline head but reads approval content only from streamed frames, so pre-existing approval content is not hydrated.

Fix commit: acbeed2ec31a6d7c2f97fc82d00e0271d904ad70.
The rendered prompt now travels through criterion diagnostics, the additive detail DTO
and the approval event; the event counts criteria rather than JSON bytes. Web hydrates
the current generation/gate from durable detail and fences the stream fallback by identity.
The canonical evaluator, store and run-page suites cover those boundaries. A fresh mobile
document shows the resolved onboarding/source-index.md question and Criteria=1, matching
loops-human-review-waiting-uds.json and loops-human-review-waiting-events.json. The
subsequent generation preserves the same actionable question.

Delivery closure: make gate passed on the frozen tree (loops-human-review-delivery-gate-v6.json). The commit hook changed no file content; loops-human-review-committed-head.json records the checked hashes and commit.
