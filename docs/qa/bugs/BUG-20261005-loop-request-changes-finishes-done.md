# BUG-20261005-loop-request-changes-finishes-done: Request changes finishes the run without applying the decision

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Marina
- **Journey Step:** J-03, find and decide a waiting human gate
- **Scenarios:** LP-010; LP-009
- **Found:** 2026-10-05 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

The reviewer requests changes to a human gate, but the run is declared done in the same generation. The stored request_changes decision is never evaluated.

## Reproduction

CH-002, Interrupt Tour, 430x932 touch viewport at DPR 3, 4G, en-US.
Run studio-onboarding-review, wait for review_source_index, then tap Request changes. Independently repeat POST /approve with decision=request_changes. Both public runs end done at generation 1 instead of creating a gate_revise successor.

## Evidence

Web run looprun-63af02cfba86f991 and independent API run looprun-3b3e1556a1cf3dc7 reproduce. loops-marina-revise-events.json contains one node_wait_resumed decision=request_changes followed by done; loops-marina-revise-api-* retains independent HTTP/UDS evidence.
All receipts are under docs/qa/evidence/2026-10-02-untested/. The exact twelve-frame
loops-review-marina recording is closed. No provider workers or mocked services are involved.

## Fix

claimActiveApprovalWait claims the durable wait and reactivates the coordinator, but leaves the gate output succeeded. Control evaluation only runs pending outputs, so the finisher skips the recorded decision.

Fix commit: acbeed2ec31a6d7c2f97fc82d00e0271d904ad70.
The approval transaction rearms the matching gate epoch; reevaluation settles the awaiting
projection once while retaining append-only observations. The existing global store
wait/history suite covers epoch fencing, settlement and final-verdict conflicts.
Fresh run looprun-b6d9838c4fb9f0b9 routes request_changes to generation 2, then reaches Done
after a single approve. Independent UDS and durable events confirm both accepted decisions:
loops-human-review-revision-uds.json, loops-human-review-done-uds.json and
loops-human-review-final-events.json. Reject remains Blocked in looprun-b5fe6bf9d8bf8b7c.

Delivery closure: make gate passed on the frozen tree (loops-human-review-delivery-gate-v6.json). The commit hook changed no file content; loops-human-review-committed-head.json records the checked hashes and commit.
