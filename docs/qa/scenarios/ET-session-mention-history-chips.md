---
id: ET-session-mention-history-chips
area: ET
title: Sent mentions render as chips after reload and old messages are unchanged
persona: Rafa
journey:
expected: After reload, a user message with verified mentions renders file, folder (`/`), and session-title chips at their offsets from `metadata.mentions`; clicking a session chip opens that session; file and folder chips only show a path tooltip; an unresolved token renders as plain text; a session renamed after send keeps the recorded title; messages sent before the feature render exactly as before; copying the message copies the raw token text.
entry_points: web session window transcript; GET /api/workspaces/{workspace_id}/sessions/{session_id}/transcript; compozy session history --json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-session-mention-file; ET-session-mention-session
---

US-008. Send `See @README.md @docs/ @session:<A> @missing.go` and reload the window: the bubble shows
`README.md`, `docs/`, and A's title as chips and `@missing.go` as plain text. The transcript API's user
message has `metadata.mentions` with three entries and relative targets only. Hover a file chip for its
path; click it and confirm nothing opens. Click the session chip: session A opens. Rename A, reload,
and confirm the chip still shows the old title. Select and copy the message: the clipboard holds the
raw token text. Open a session with messages from before the upgrade and confirm their rendering is
unchanged.
