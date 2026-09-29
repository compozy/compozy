---
id: RT-conversation-rewind
area: RT
title: Rewind an idle conversation and continue from its retained prefix
persona: Théo
journey: J-rewind-conversation
expected: The same session keeps the retained prefix, restores the selected prompt as a draft, continues with fresh provider context, and exposes the discarded suffix only through archived reads; a child created with --parent (lineage kind provenance) rewinds the same way while a spawned child is still refused as daemon-managed
entry_points: Web session thread; compozy session rewind; POST /api/workspaces/:workspace_id/sessions/:session_id/rewind
qa_status: untested
bug_ids: BUG-20260805-rewind-reader-unavailable
fix_status: verified
retest_status:
fix_commits: 6c8deff
evidence:
last_report:
overlaps: ET-web-session-fork-from-here
---

Conversation rewind does not restore files, tool effects, network calls, or memory. The confirmation and structured output must preserve that boundary.

Provenance child case: create a session with `compozy session new --parent <root-id>`, prompt it, confirm
`compozy session status <child-id> -o json` reports `lineage.kind: "provenance"`, then rewind the child
to its first prompt. The rewind succeeds with the same child session ID and the parent is untouched. A
spawned child (`lineage.kind: "spawn"`) still returns `409` as a daemon-managed session.

Web note (session-continue-fork task_06): a durable user message now shows "Fork from here" right
before "Rewind to here", with the same availability and busy gates (thread running, a pending rewind,
or rewind blocked disables both). Walk that both disable together and that forking leaves this
session's transcript untouched; the fork itself is walked in ET-web-session-fork-from-here.
