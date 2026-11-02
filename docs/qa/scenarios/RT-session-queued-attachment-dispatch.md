---
id: RT-session-queued-attachment-dispatch
area: RT
title: Dispatch queued prompt attachments
persona: Théo
journey: J-session-attachments
expected: A busy-session prompt keeps its attachment refs with the durable queued input and dispatches the same ordered attachments once when the queue entry is delivered; reload and native history/events keep metadata only.
entry_points: HTTP+UDS prompt; native session_prompt; CLI session input; persisted events
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-15-session-attachments-pr-412-final/07-busy-attachment-ready.png; docs/qa/evidence/2026-08-15-session-attachments-pr-412-final/08-queued-attachment.png; docs/qa/evidence/2026-08-15-session-attachments-pr-412-final/09-queued-attachment-delivered.png
last_report: docs/qa/reports/2026-08-15-session-attachments-pr-412-final.md
overlaps: RT-019
---

QA impact 2026-11-02 (responsiveness audit): the Put traversal reduction preserves a fresh pin
lookup before reserving capacity, so a newly pinned queued attachment cannot be evicted by the
second retention pass. Open still validates the bytes later consumed by prompt dispatch, now
using one file descriptor. Durable queue ordering, admission identity and byte-free metadata
are unchanged; production pin queries remain per session and were not cached or batched.

Focused evidence: [terminal performance audit](../reports/2026-11-01-performance/terminal-terminal.md).
`TestFilesystemAttachmentStoreRetention/Should refresh pins again before reserving capacity for a new attachment`
and `Should preserve pending-input pins until the queue releases them` passed with the filesystem
suite and `-race`. The new freshness case controls the pin-source I/O boundary; it does not claim
an end-to-end busy-session dispatch or daemon restart was walked.

Existing runtime/persistence CI targets, identified here but not executed by this note:

- `internal/daemon/daemon_test.go`: `TestBootSessionAttachmentRetentionPinsQueuedInputs`
  exercises startup retention with real attachment files and a controlled queue dependency.
- `internal/store/globaldb/global_db_session_input_queue_test.go`:
  `TestGlobalDBSessionInputQueueGeneration/Should preserve attachments through queue insert and lease`
  and `TestGlobalDBSessionPromptAdmission/Should persist and replay admission attachments` use real SQLite.
- `internal/session/prompt_attachments_test.go`: `TestSendPromptDispatchesAttachments`
  covers resolved dispatch, replay without a duplicate driver call and retry before dispatch;
  `TestResolvePromptAttachments` owns digest/capability rejection at the attachment I/O boundary.

No dedicated queued-attachment Playwright journey exists in the current browser suite.
`web/e2e/__tests__/session-hardening.spec.ts` has
`E2E-012 / E2E-025: composer honors default steering, explicit queue actions, and persisted follow-up settings`,
but it queues text and cannot certify attachment retention/order. Root still owns the busy-session
attachment/reload/dispatch walk and its receipt. The historical verdict above is not re-certified.
