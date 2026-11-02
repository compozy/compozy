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
