---
id: RT-sdk-stream-recovery
area: RT
title: Recover a transcript after the reader disconnects
persona: Ana
journey: J-build-external-app
expected: A dropped event stream resumes after the last consumed event while prompt disconnect never resubmits or cancels work, and explicit cancellation remains a separate operation.
entry_points: Compozy.events; Compozy.prompt; Go StreamSession; durable transcript
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-session-prompt-cancel
---

Record event IDs from a live session subscription, interrupt the transport, and reconnect. Confirm Last-Event-ID and the latest transcript epoch/generation query fences recover only later frames without duplicate transcript content. Abort during reconnect backoff and verify no further requests. Drop a prompt stream mid-frame and after a complete text frame without finish/abort/[DONE]; observe a stream error in both cases, and recover the completed result from the transcript without a second turn. Confirm 4xx is terminal, stream.consumer_degraded reaches the app, and explicit prompt cancel ends the turn while reader cancellation alone does not.

QA impact 2026-10-17: App SDK implementation adds this user-visible journey. Authored planning only;
no live execution verdict is recorded. Execute in the assigned isolated lab and attach current-head
evidence before marking it passed.
