---
id: ET-session-attachment-paste-reload
area: ET
title: Paste, send, and reload a session image
persona: Théo
journey: J-session-attachments
expected: Pasting one supported image into the session composer shows a removable preview; sending succeeds, and a cold reload renders the same attachment from persisted event metadata and the scoped file route.
entry_points: web session composer; session transcript reload
qa_status: pass
bug_ids: BUG-20260815-session-attachment-store-unavailable
fix_status: fixed
retest_status: pass
fix_commits: 2603eed
evidence: docs/qa/evidence/2026-08-15-session-attachments-pr-412-final/10-pasted-image-ready.png; docs/qa/evidence/2026-08-15-session-attachments-pr-412-final/11-pasted-image-reloaded.png; qa-artifacts/qa/api-attachment-bytes.png
last_report: docs/qa/reports/2026-08-15-session-attachments-pr-412-final.md
overlaps: ET-web-session-composer-text-entry
---


## Image preview return — #738

At desktop size and a narrow viewport, open an image from both the draft composer and
its sent transcript. The preview must show the filename, full image, and visible Close
button. Close it with the button, Escape, and a click on the backdrop; each must return
focus to the initiating preview control and preserve the session URL. Reopen it after
every dismissal. The composer keeps the attachment and draft text; the transcript keeps
the conversation. An image inside the viewer does not navigate. Repeat in an OS-window
portal and verify a small window keeps the Close button reachable. Text/PDF and other non-image file links
download or open externally without navigating the current session. Reload must continue to read the scoped bytes URL.

Targeted verification and screenshots for this repair are recorded in
`docs/qa/reports/2026-10-16-issue-738-image-preview.md`; the historical pass above covers
the earlier persistence journey and does not imply a fresh daemon or Linux app run.

## Unsent attachments across session switches — #922

In session A, type text, paste two images, and attach a text file. Switch the same
window to session B from the sessions sidebar and back. Session A's composer must show
the text and all three tiles in their original order, ready, with image previews; the
switch must not upload the files again or delete them. Remove one tile, switch away and
back: it stays removed. Send the draft: the prompt carries the original attachment refs
and no new upload; switching away and back afterwards shows an empty composer. Session B
never shows session A's files. A full page reload starts from the text draft only;
unsent files are retained in browser memory for the app run, not across reloads.

Walked 2026-10-23 against an isolated daemon with the Vite app (provider-free acpmock
agent): before the fix the text returned and every tile was gone
(https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-922/before-03-after-switch-back.png);
after it the draft returned whole
(https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-922/03-session-a-draft-restored.png)
with three uploads total, and the removal and send checks above held.

QA impact 2026-10-23 (merge train): deleting a session after switching away must release its
retained unsent files and upload handles. Verify the deleted draft cannot restore attachments;
other sessions' drafts remain available. Covered in the existing session deletion hook suite.
