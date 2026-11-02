---
id: ET-session-attachment-multiple-drop
area: ET
title: Drop multiple session attachments
persona: Théo
journey: J-session-attachments
expected: Dropping multiple supported files adds one preview per file, preserves their order, sends all refs in one prompt, and renders every attachment in the persisted transcript.
entry_points: web session composer drop target; session transcript
qa_status: pass
bug_ids: BUG-20260815-session-attachment-store-unavailable
fix_status: fixed
retest_status: pass
fix_commits: 2603eed
evidence: docs/qa/evidence/2026-08-15-session-attachments-pr-412-final/12-drop-overlay.png; docs/qa/evidence/2026-08-15-session-attachments-pr-412-final/13-multiple-drop-ready.png
last_report: docs/qa/reports/2026-08-15-session-attachments-pr-412-final.md
overlaps: ET-session-attachment-paste-reload
---

QA impact 2026-11-02 (responsiveness audit): a new attachment Put reuses one filesystem inventory
under the existing mutation lock while both retention passes still refresh pending-input pins.
Open verifies content and metadata from the same file descriptor returned to its caller, then
rewinds it. Count/age/byte retention, digest/MIME/dimensions checks and workspace/session scope
remain authoritative; there is no persistent metadata/content cache or change to upload ordering.

Focused evidence: [terminal performance audit](../reports/2026-11-01-performance/terminal-terminal.md).
The real temporary-filesystem suites `TestFilesystemAttachmentStoreRoundTrip`,
`TestFilesystemAttachmentStoreRetention`, `TestFilesystemAttachmentStoreMetadataIntegrity` and
all other `TestFilesystemAttachmentStore*` cases passed, followed by the final `-race` rerun
(1.873 s). These prove storage behavior and corruption/scope rejection, not multiple-file browser
preview order or the final transcript rendering.

Existing CI target: `internal/api/core/session_attachments_test.go`, `TestSessionAttachmentHandlers`,
including `Should upload and read attachment bytes with stored metadata` and
`Should reject a foreign workspace before touching storage`, exercises the HTTP handler boundary
with real attachment storage. This target is identified, not newly executed by this note.

No dedicated multiple-file drop Playwright test exists in the current `web/e2e/__tests__` tree.
Root's isolated browser walk must still drop at least two supported files, preserve preview order,
send both refs together, then reload and read each persisted attachment. No new browser pass is
claimed, and the historical verdict/evidence above remains unchanged until that receipt is added.

QA impact 2026-11-02 (#1142): add the same PNG twice, remove one tile, switch
sessions and return, then send the remaining image. Removing a tile must not
DELETE content-addressed bytes shared with another tile or persisted message.
Repeat removal while upload is in flight. Daemon retention owns abandoned bytes.

Recovery probe: delete the uploaded image through the public attachment API in
an isolated lab before Send. The refused prompt must keep its text, re-upload the
retained File, and offer Send again with readable recovery copy (no JSON envelope).
Retry must persist exactly one user message with the image. Repeat at narrow and
wide composer widths; no operator runtime/session may be used for this probe.

Focused proof: adapter duplicate/rebuild/removal cases and chat-runtime provider
missing-image recovery/resend integration pass. A browser/daemon receipt for this
new probe is required before replacing the historical QA verdict above.

Rendered QA follow-up (#1142, initial fix): the orchestrator exercised 1118px and
399px viewports. Duplicate removal, in-flight removal without resurrection,
ready-plus-in-flight duplicate removal, draft restoration, exact inline recovery
copy and multi-file drop passed. Retry failed visually: the daemon recorded one
user message, but the refused optimistic bubble remained with a broken image.
The runtime now evicts that refused local message by ID, dismisses the session's
refusal toast on acceptance, and removes uploading tiles immediately. Repeat
recovery and retry at both widths; require zero refused bubbles before retry,
exactly one bubble after acceptance, preserved draft text and no stale toast.
These changed states remain pending rendered re-verification.

The lab enabled image capability on acpmock using a build overlay. The mock records
text only, so daemon admission, persisted file metadata and byte reads were
verified; image delivery to the mock agent was not directly observed. Multi-drop
used a dispatched DataTransfer rather than an OS drag. Strict audit C12/C14 are
report/gate artifact blockers, not observed behavior failures. The send-feedback
wrapper follows COPY.md's composer send feedback contract and is retained.

Review follow-up: the adapter also registers upload ownership before yielding the
first running tile, covering removal before the next generator read. Its existing
suite reproduces the earlier resurrection gap and covers both sides of that yield.
Refused-message eviction awaits and observes runtime deletion failures while the
draft remains recoverable; the recovery-owner suite covers that failure boundary.

Second rendered re-walk at 1118px and 399px passed normal send, 5xx handling,
duplicate/in-flight removal and recovered retry without stale bubbles, broken
images, toast or tiles. S3 still failed because deleting the refused message also
removed the inline error, leaving only a temporary toast. The reason now belongs
to the restored draft and reuses the existing composer feedback note, with wrapping
copy at narrow widths. It survives toast dismissal and session reopening, and
clears on acceptance, attachment removal or draft discard. Final S2/S3/S4 remain
pending rendered re-verification on the new head; the integration suite covers
all four notice lifecycle paths without relying on the toast timer.

Feedback follow-up: the refusal is published before re-uploading the recovered
files. Removing a recovered tile while that upload is pending clears the notice
once, and late upload completion cannot publish it again. New Queue/Steer/Interrupt
results replace the old prompt notice; acceptance clears it even if the operator
rewrote the draft while waiting. Existing provider and composer suites reproduce
these races and cover the fixes. The multipart-file boundary also verifies that
recovery uploads the retained image name and bytes, rather than only counting POSTs.

Rendered walk at d8570b2de passed S1, S2, S3 and its end conditions, S4, normal
send and ambiguous 5xx at both widths. A separate long-token probe still overflowed
at 399px; the existing feedback note now permits overflow wrapping for URLs/paths.
The final delta must verify that token, removal during re-upload without stale
advice returning, newer busy-send results replacing old guidance, and S3/S4 smoke.
No final-head visual verdict is claimed until that delta is complete.

Full-page reload remains a known pre-existing limit: main's draft localStorage
envelope stores text only; retained Files live in module Maps for one app run.
Reload therefore loses unsent tiles and the memory-only refusal reason, leaving
text that can be sent alone. Session reopening in the same app run is covered;
durable attachment drafts or a persisted loss marker need a separate contract.
Main was compared by source, not by a separate rendered walk. The transient global
refusal toast can outlive attachment removal or session navigation until its
timeout, though acceptance dismisses it. The optimistic bubble can flash before
authoritative refusal and request deliberately deleted image bytes; it must be
gone after refusal and remain absent after a successful retry.
