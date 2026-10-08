---
id: RT-conversation-rewind
area: RT
title: Rewind an idle conversation and continue from its retained prefix
persona: Théo
journey: J-rewind-conversation
expected: The same session keeps the retained prefix, restores the selected prompt as a draft, continues with fresh provider context, and exposes the discarded suffix only through archived reads; a child created with --parent (lineage kind provenance) rewinds the same way while a spawned child is still refused as daemon-managed
entry_points: Web session thread; compozy session rewind; POST /api/workspaces/:workspace_id/sessions/:session_id/rewind
qa_status: untested
bug_ids: BUG-20260805-rewind-reader-unavailable; BUG-20260929-rewind-draft-lost-on-row-unmount; BUG-20260929-rewind-offered-after-rewind
fix_status: fixed
retest_status: pass
fix_commits: uncommitted (review round 1 QA re-walk)
evidence: docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/w-rewind-retest-after.png; docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/rewind-retest.txt
last_report: docs/qa/reports/2026-09-29-session-continue-fork-r1-rewalk.md
overlaps: ET-web-session-fork-from-here; RT-session-lineage-upgrade
---

Conversation rewind does not restore files, tool effects, or network calls. The confirmation and structured output must preserve that boundary.

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

## 2026-09-29 web leg (review round 1) — PASS (after fix)

Théo, lab `…-r1-rewalk-…`, acpmock `lab-runner`, 1440×900.
- Busy gate: while an API-started `block until canceled` turn runs, "Fork from here" and "Rewind to here" on a settled user message are both disabled. After `compozy session prompt-cancel` both are enabled.
- Rewind: the confirmation reads "Rewind this session? … Files, tool effects, and saved memory will not be undone." Confirming cut the same session (epoch 0 → 1, prefix kept), but the composer stayed empty (BUG-20260929-rewind-draft-lost-on-row-unmount, fixed). Retest on a fresh session: the composer holds "Second step". Sending it got "Second step done." on the same session id (fresh ACP context). The transcript is the prefix plus the new turn.
- Fork leaves the source untouched (ET-web-session-fork-from-here re-walk: `max_sequence` unchanged).
- Found, pre-existing, open: BUG-20260929-rewind-offered-after-rewind. After a rewind followed by new turns, "Rewind to here" stays enabled but the daemon refuses it (`400 conversation rewind target is invalid`, documented refusal), and the web shows a generic "Refresh the conversation" toast.
The web leg is closed here; B1/C own the CLI and provenance legs, and the archived-read boundary was last walked in `2026-08-04-session-rewind.md` (unchanged by this branch). Evidence: `docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/w-busy-gate-running.png`, `docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/w-busy-gate-idle.png`, `docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/w-rewind-confirm.png`, `docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/w-rewind-after.png`, `docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/w-rewind-retest-after.png`, `docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/rewind-retest.txt`.

QA impact 2026-10-07 (memory removal): the Web confirmation copy dropped its memory fragment and now reads "Rewind this session? … Files and tool effects will not be undone."; the quote in the 2026-09-29 walk above records the earlier text. Stale verdict reset to untested; historical evidence preserved; no QA session ran.
