---
id: NB-indeterminate-bridge-delivery
area: NB
title: Handle indeterminate bridge delivery without replay
persona: Omar
journey: J-connect-bridge-provider
expected: "A real bridge send reports `delivered` only with a non-empty provider remote ID. Every provider ACK is an object with explicit non-empty `delivery_id` and integer `seq` matching the request: explicit `seq: 0` is valid, while missing, null, wrong-type, or mismatched identity is invalid. A mutation that may have committed but whose ACK or required result cannot be materialized reports `committed_result_unavailable` with a redacted error and no remote ID, is never replayed automatically, and leaves later independent text eligible after an indeterminate progress update. Credential-bearing provider clients return the original `3xx` without forwarding credentials or replaying mutation bodies to a redirect target."
entry_points: agh bridge send-test; HTTP and UDS bridge send-test; Web send-test dialog; fake-provider delivery transports
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: NB-long-bridge-replies; NB-provider-progress-rendering; NB-bridge-provider-setup; NB-bridge-restart-recovery
---

An operator receives a truthful terminal outcome when a provider mutation may have committed but its acknowledgement cannot be materialized, without risking an automatic duplicate delivery.

Phase D impact flag 2026-07-13: R-001 hardens write-ahead delivery intent, semantic ACK identity, response materialization, provider redirect refusal, and truthful API, CLI, and Web send-test outcomes. Planning flag only; no QA retest ran.
