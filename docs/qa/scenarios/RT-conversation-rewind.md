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
overlaps: ET-web-session-fork-from-here; RT-session-lineage-upgrade
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

2026-09-28 (session-continue-fork task_08 part C, Lab B upgrade walk): pre-feature leg walked on the
branch build over a home written by `67b86a9b9`. The pre-feature `--parent` child
`sess-e2840770f780c14b` (`lineage.kind: "provenance"` from the backfill) rewound to its first prompt:
exit 0, same id, `transcript_epoch 1`, `draft_text "hello child"`, parent transcript unchanged. The
pre-feature spawned child `sess-b855be48db40d34d` (`spawn`) was refused: CLI exit 65 and HTTP `409`
`managed sessions cannot be rewound`. Evidence: `docs/qa/evidence/2026-09-28-session-continue-fork-upgrade/`.
Verdict stays with the Lab A walk: the web leg and the branch-created `--parent` child leg were not
walked here.

2026-09-28 (session-continue-fork task_08 part B1): branch-created provenance leg walked (CLI/API, acpmock). `session new --parent
sess-f5db2af4fddaeb77` child `sess-4b36c06c36de3ded` reports `lineage.kind: "provenance"`; after two turns, `session rewind <child>
--message-id <first> --expected-*` succeeded with the same session id, the child's epoch moved to 1 with no active user message, and the
parent's fences were unchanged. A child spawned by a real Claude session through `compozy__session_spawn` (`lineage spawn`) is refused:
HTTP `409` `managed sessions cannot be rewound`. Evidence: `docs/qa/evidence/2026-09-28-session-continue-fork-b1/prov-rewind.json`, `docs/qa/evidence/2026-09-28-session-continue-fork-b1/rewind-spawned.txt`. The web leg belongs to part B2.
