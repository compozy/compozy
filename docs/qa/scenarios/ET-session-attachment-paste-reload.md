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
