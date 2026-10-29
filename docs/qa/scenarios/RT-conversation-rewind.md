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

Conversation rewind does not restore files, tool effects, or network calls. The composer edit notice and structured output must preserve that boundary.

Rewind starts fresh provider context, so it also discards a resume replay that is still pending: after a runtime
replacement followed only by a maintenance turn, a rewind must not let the pre-rewind replay block reappear on
the next ordinary prompt (only the rewind's own replay of the retained prefix reaches the agent). On a home upgraded from the release that archived compacted spans, events archived by a rewind
stay archived while the span the removed compaction had archived is visible again (RT-upgrade-memory-removal-home).

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

QA impact 2026-10-07 (memory removal): the Web confirmation (`session-rewind-message-action.tsx`) dropped its memory fragment and now reads the title "Rewind this session?" with the description "Messages from this point onward will be removed from the active conversation. Files and tool effects will not be undone."; the quote in the 2026-09-29 walk above records the earlier text. Stale verdict reset to untested; historical evidence preserved; no QA session ran.

Issue #872: the Web action is labeled **Edit message** and explains that the selected message returns to the composer while its conversation suffix is removed. Stop an active response, wait for cancellation to settle, edit the sent message, change its text, and send again in the same session. Verify the corrected response and absence of the discarded request from the active transcript. A pre-existing draft and busy/decision fences must still prevent destructive replacement. CLI/API rewind naming is unchanged. Owning browser coverage: `session-hardening.spec.ts`.

QA 2026-10-21 (#872): the scoped Web stop/edit/resend leg passed in the isolated browser runtime with the real daemon and ACP mock provider, followed by clear/reload/delete checks. The selected text returned to the composer, its accessible name stayed stable through the reset, and resending produced a new response in the same session. Historical CLI/provenance coverage above was not rerun.


Issue #995 supersedes the Web confirmation/draft guard above. **Edit message** now selects
text locally in the official composer, replacing any existing draft and its attachments.
No dialog or rewind request occurs on selection. The composer identifies edit mode and explains
that sending replaces this message and later replies without undoing files or tool effects.

1. In an idle or stopped user session, type an unsent draft and select a durable message's Edit action.
   Verify exact multiline text, focus in the composer, no dialog, and unchanged active history.
2. Cancel through **Cancel edit**, Escape, and clearing the text; verify the conversation is unchanged.
3. Select again, change the text, and submit with Enter and the send button. Verify one rewind before
   one prompt, the retained prefix, exactly one replacement message, and a fresh response in the same session.
4. Hold the rewind response while the transcript removes the original row; the composer still completes
   the send once. Repeated sends while pending must not create another rewind or prompt.
5. Refuse rewind (for example an open decision or stale fence); verify the daemon's reason is shown,
   the edited text remains available, and no replacement prompt is sent.

Owning suites: `session-chat-runtime-provider.test.tsx`, `session-thread.test.tsx`, and the
stop/edit/resend journey in `session-hardening.spec.ts`. Fork keeps its own existing gates.
QA 2026-10-27 (#995): both canonical component suites passed (291 tests). The isolated
browser journey passed against the real daemon and ACP fixture provider: stopped-session
selection replaces the draft and focuses the editor without a dialog; Escape preserves history;
reselection and Enter replace the original message, receive a new response, and retain the
existing clear/reload/delete checks. Component coverage verifies button submission, multiline
text, cancellation by button/clearing, transcript-row removal during rewind, and daemon refusal
with the draft retained. Screenshots: `session-edit-selected` and `session-stop-edit-resend`
in the Playwright browser artifacts. Historical CLI/provenance legs above were not rerun.

Review re-walk 2026-10-27 (#995): the browser journey now holds the real rewind response,
switches to another session, releases the response, and verifies the original session receives
its replacement and response before returning. Both focused runtime/mutation suites passed
(120 tests), including exact rewind request replay after a post-acceptance refetch failure.
The submitted text and attachment references survive navigation through the existing prompt
transport; an uncertain rewind retry retains its original idempotency key and daemon-issued
transcript fence. A definite refusal permits a refreshed fence on the next submission.

## Lost replacement acknowledgment after navigation (#1017)

Extend the stop/edit/resend journey by holding the accepted rewind response, navigating to
another session, and then releasing it. Forward the detached replacement prompt to the real
daemon but drop its acknowledgment. After the replacement response is durable, return to the
original session: the composer must be empty and the unconfirmed row must offer **Retry**.
Retry must submit the original request, including its message identity and idempotency key.
The daemon must report `replayed: true`; the unconfirmed marker disappears and the composer
stays empty. The accepted replacement must not create another turn. A definitive refusal still
preserves the draft, and a newer draft must never be discarded by the earlier send's completion.

Owning regression: the existing stop/edit/resend test in
`web/e2e/__tests__/session-hardening.spec.ts`; it uses Chromium and an isolated daemon serving
`web/dist`, with the existing ACP fixture provider. Only the acknowledgment is disrupted.

Verification 2026-10-29: the targeted browser journey failed on main
`44b3e6dce6db9e218f799d211b8fb05229c46c9c` because the composer retained
`continue after disconnect` alongside Retry. After the hook fix, the same journey passed
(1 test, 11.3s execution), including the replay receipt and existing clear/reload/delete legs.
The mounted-guard effect did not need changing for this reproduction. Focused recovery, runtime,
and action suites passed (131 tests); web typecheck passed and React Doctor scored 100/100.
Browser artifacts include `session-edit-safe-retry` and `session-stop-edit-resend`. Historical
CLI/provenance and live-provider journeys were not rerun.
