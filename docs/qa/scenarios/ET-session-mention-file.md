---
id: ET-session-mention-file
area: ET
title: Mention a file from the @ picker and the agent receives a link
persona: Théo
journey:
expected: Typing `@clie` in a session composer opens the picker with "Files" and "Sessions" groups; picking `client_prompt.go` inserts a chip; the sent text is `@internal/acp/client_prompt.go`; the agent's session/prompt carries the text block then one resource_link (name relative, uri absolute file:// under the session working directory) and no file contents; the builtin Claude and Codex adapters show the model a `[@…](file:///…)` link after the literal path; attachment blocks come after the link; the agent opens the file; the transcript shows the chip; `.gitignore`d files are not listed; more than 25,000 files show the "Showing results from the first 25,000 files" footer; outside git the picker walks the folder skipping `.git` and `node_modules` without applying `.gitignore`.
entry_points: web session window composer; GET /api/workspaces/{workspace_id}/sessions/{session_id}/files; compozy session events --json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-session-mention-history-chips; ET-session-mention-folder-quoted; ET-session-mention-dismiss
---

Golden path (E2E-001, US-001, US-002, US-005, US-001.EC-4/EC-5/EC-8). In a real Claude session and again in a real Codex
session: click the composer and confirm the placeholder "Send a message — type / for commands, @ to
mention". Type `Review @clie`: the picker opens above the composer, focus stays in the editor,
"Files" lists `client_prompt.go` (secondary `internal/acp`) above deeper matches, and "Sessions"
reads "No matching sessions" when nothing matches. ↑/↓ wrap; Enter picks. The chip reads
`client_prompt.go` and its tooltip shows `internal/acp/client_prompt.go`; Backspace right after it
removes the whole chip (re-pick before sending). Dismissing and ignoring the picker is walked in
ET-session-mention-dismiss.

Send. In `compozy session events <id> --json`, the delivered prompt shows the text block with the
literal token, then one `resource_link` `{name: "internal/acp/client_prompt.go", uri:
"file:///<working dir>/internal/acp/client_prompt.go"}`, and the delivery record counts `links: 1`.
No embedded resource appears whatever `embeddedContext` the agent advertises. The agent answers
about the file's real content. Open a bare `@` and confirm the file now appears first as a recent
mention. Failure leg: remove the session's working directory and confirm the Files group reads
"Couldn't search files" while typing and sending still work.

Model view (ADR-002): confirm the builtin adapters turn the link into Markdown text for the model.
In the adapter's own transcript, find the user turn and search it for `file://`. Claude writes
`<CLAUDE_CONFIG_DIR or ~/.claude>/projects/<working dir with / as ->/<session>.jsonl` and shows
`[@client_prompt.go](file:///<working dir>/internal/acp/client_prompt.go)` (the last path segment).
Codex writes `<CODEX_HOME or ~/.codex>/sessions/<yyyy>/<mm>/<dd>/rollout-*.jsonl` and shows
`[@internal/acp/client_prompt.go](file:///<working dir>/internal/acp/client_prompt.go)` (the link
name). Both follow the literal `@internal/acp/client_prompt.go` already in the text. When the
transcript file is not reachable, ask the agent to quote verbatim every line of the message that
contains `file://` and compare.

Attachment order (US-005.AC-4): send `Compare @internal/acp/client_prompt.go with this` with a
Markdown file attached. The delivered prompt lists the text block, then the `resource_link`, then the
attachment block last.

Picker source legs (US-001.EC-4/EC-5/EC-8). After changing files, wait 15 seconds so the cached
index expires. (1) Git: with `node_modules/left-pad/index.js` and `.env` present and ignored by
`.gitignore`, `@left-pad` and `@.env` read "No matching files", while an untracked, not-ignored
`notes.md` is listed. (2) Over 25,000 files: generate 25,100 files under `gen/`; a bare `@` and a
`@gen` query end the Files group with "Showing results from the first 25,000 files", and the search
route returns `truncated: true`. (3) Outside git: create a session whose working directory is a plain
folder (no `.git` anywhere above it) holding `node_modules/x.js`, `secret.txt`, and a `.gitignore`
that ignores `secret.txt`. `@x.js` finds nothing under `node_modules`, while `@secret` lists
`secret.txt` because `.gitignore` is not applied when walking.
