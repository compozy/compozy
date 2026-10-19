---
id: ET-session-mention-containment
area: ET
title: Mentions never escape the receiving session's working directory or workspace
persona: Théo
journey:
expected: `@../sibling/x`, `@/etc/hosts`, a symlink inside the working directory pointing outside it, `@session:<id>` of another workspace, of another profile (whose title never appears in the delivered prompt), of the session itself, or of a deleted session, and `me@example.com` never become mentions — no chip, no resource_link, no reference, and the prompt is still sent; a worktree-bound session resolves and searches inside its worktree, and a session with a creation-profile subdirectory resolves and searches inside that subdirectory; the file search route returns 404 session_not_found for a session of another workspace, 400 invalid_request for limit=0, and 409 session_directory_unavailable when the working directory is gone; a queued link whose file becomes an outward symlink before dispatch is dropped.
entry_points: compozy session prompt; web session window composer; GET /api/workspaces/{workspace_id}/sessions/{session_id}/files over HTTP and UDS; compozy session events --json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-session-mention-queued-input
---

US-009 (incl. AC-2 other profile), US-005.EC-1/EC-5, US-001.AC-4, US-001.EC-9. Use a workspace with a session bound to a worktree, a sibling
directory, and a second workspace. Create `link-out` inside the working directory pointing to a file
outside it. Send through the CLI: `Check @../sibling/a.go @/etc/hosts @link-out
@session:<other-workspace-session> @session:<self> @session:<deleted> mail me@example.com`. The prompt
is accepted; the delivered prompt has the text only — no resource_link and no references block — and
history shows no chips. `@sub/../a.go` that stays inside resolves with target `a.go`.

Other-profile leg (US-009.AC-2): create session P in the same workspace under a second profile and
give it a distinctive title such as `QA-PROFILE-CANARY`. From a session of the default profile send
`Read @session:<P>`. The prompt is accepted; the delivered prompt has no references block, no text
anywhere in the delivered blocks or the delivery record contains `QA-PROFILE-CANARY`, the user
message has no `mentions`, and history shows `@session:<P>` as plain text. The picker of the
default-profile session never lists P.

In the web picker for the worktree-bound session, results list the worktree's files, not the
workspace root's. Creation-profile leg (US-001.EC-9): start a session whose creation profile sets a
subdirectory working directory (for example a Loop `run-agent` node with `cwd` pointing at
`services/api`). Its picker lists `services/api`'s entries as top-level paths (`main.go`, not
`services/api/main.go`), `@main.go` resolves to `file:///<root>/services/api/main.go`, and a file
that exists only at the workspace root does not resolve. Over HTTP and UDS: `GET …/sessions/<other-workspace-session>/files` under this
workspace → 404 `session_not_found`; `limit=0` → 400 `invalid_request`; after deleting the worktree
directory → 409 `session_directory_unavailable`. Dispatch re-check: queue `@a.go` behind a running
turn, replace `a.go` with a symlink pointing outside, let the turn dispatch, and confirm no link for
`a.go` and `dropped_at_dispatch: 1` in the delivery record.
