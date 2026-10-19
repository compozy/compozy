---
id: ET-session-mention-file
area: ET
title: Mention a file from the @ picker and the agent receives a link
persona: Théo
journey:
expected: Typing `@clie` in a session composer opens the picker with "Files" and "Sessions" groups; picking `client_prompt.go` inserts a chip; the sent text is `@internal/acp/client_prompt.go`; the agent's session/prompt carries the text block then one resource_link (name relative, uri absolute file:// under the session working directory) and no file contents; the agent opens the file; the transcript shows the chip.
entry_points: web session window composer; GET /api/workspaces/{workspace_id}/sessions/{session_id}/files; compozy session events --json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-session-mention-history-chips; ET-session-mention-folder-quoted
---

Golden path (E2E-001, US-001, US-002, US-005). In a real Claude session and again in a real Codex
session: click the composer and confirm the placeholder "Send a message — type / for commands, @ to
mention". Type `Review @clie`: the picker opens above the composer, focus stays in the editor,
"Files" lists `client_prompt.go` (secondary `internal/acp`) above deeper matches, and "Sessions"
reads "No matching sessions" when nothing matches. ↑/↓ wrap; Enter picks. The chip reads
`client_prompt.go` and its tooltip shows `internal/acp/client_prompt.go`; Backspace right after it
removes the whole chip (re-pick before sending). Esc on a new `@` closes the picker for that
occurrence only; a later `@` after whitespace reopens it.

Send. In `compozy session events <id> --json`, the delivered prompt shows the text block with the
literal token, then one `resource_link` `{name: "internal/acp/client_prompt.go", uri:
"file:///<working dir>/internal/acp/client_prompt.go"}`, and the delivery record counts `links: 1`.
No embedded resource appears whatever `embeddedContext` the agent advertises. The agent answers
about the file's real content. Open a bare `@` and confirm the file now appears first as a recent
mention. Failure leg: remove the session's working directory and confirm the Files group reads
"Couldn't search files" while typing and sending still work.
