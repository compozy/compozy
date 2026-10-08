---
id: MS-workspace-checkpoint-continuity
area: MS
title: Rebuild a replaced session from one bounded replay
persona: Théo
journey: J-11
expected: When a session moves to a new agent process through account fallback or runtime or model replacement, its first prompt carries one replay bounded by `[session.derive] max_replay_bytes`: the header says the workspace files and git state are authoritative, the earliest user message is pinned ahead of the `[N earlier messages omitted to fit the context budget]` note, the last 8 messages that fit are protected, and the `compozy__session_history` pointer line appears only when messages were omitted and the session can call that tool. A replay deferred because the first accepted turn was a maintenance turn (Compact now) is a durable obligation: it survives that turn, a stop and daemon restart, and a native resume, reaches the agent exactly once with the next ordinary prompt, and is discarded by clear or rewind. No checkpoint file is written or injected, and another workspace never contributes a fact.
entry_points: daemon session reactivation through account fallback or runtime or model replacement; compozy session history; compozy session events; compozy logs (session.replay.bounded); acpmock prompt capture
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-ms-wave2-current-20260730-061842-796290-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: RT-session-context-rebuild; RT-pressure-context-compaction; RT-session-fallback-chain
---

Use two workspaces with deliberately similar content. In the first, drive a long acpmock session (about
400 turns, so the persisted transcript exceeds the 131072-byte default budget) whose first user message
states a unique fact and whose last turns state another. Move it to another account route (account
fallback) and, in a second pass, replace its runtime or model. For each rebuild capture the first prompt
the new agent process received and confirm:

- the prompt is at most the budget, starts with the replay header including "The files and git state in
  the workspace are authoritative; inspect them before acting on the transcript.", and wraps the
  transcript in `<compozy_context_replay>`;
- the earliest user message is present ahead of the omission note, the note reports how many earlier
  messages were dropped, and the last 8 messages that fit are intact;
- the pointer line `Earlier messages were omitted. Read them with the compozy__session_history tool
  (session_id: <id>) when you need them.` appears, and does not appear when the transcript fits the budget
  or when the session's tool policy denies `compozy__session_history`;
- one `session.replay.bounded` log record per rebuild that omitted messages, with `omitted_count` and
  `first_user_pinned: true`;
- the session keeps answering, and asking for the pinned first-message fact and for a fact inside the
  protected tail both succeed; lowering `[session.derive] max_replay_bytes` yields a smaller replay (below 16384 also lower `max_message_bytes`, which validation requires to stay between 1024 and `max_replay_bytes`);
- no `project_checkpoint_summary.md` appears in either workspace and no checkpoint block reaches the
  agent; nothing from the second workspace appears in the first workspace's replay.

Deferred replay (durable `pending_resume_replay` obligation). Replace the runtime of a session whose
transcript needs replay, then BEFORE any ordinary prompt run `compozy session compact <id>` (or Compact now in
the Web). Confirm the compact turn carries only `/compact` (no replay, no startup instructions), then repeat the
variants: (a) send an ordinary prompt directly after the compact turn; (b) stop the session and restart the
daemon, with an agent that supports `session/load`, and send an ordinary prompt (exactly one native load, no
rebuild from maintenance rows); (c) run `session clear` or a rewind while the replay is still pending and send an
ordinary prompt. In (a) and (b) the first ordinary prompt carries the bounded replay and startup instructions
exactly once and the next ordinary prompt carries neither; in (c) the discarded replay never reappears.

Account fallback and runtime replacement keep their existing lifecycle events and `Context rebuilt from
log.` marker. Continue and fork use the same bounded replay and have their own scenarios; a nested continue or fork keeps the
omission count and truncation evidence it inherited in the carried-context metadata and receipt.

QA impact 2026-10-07 (memory removal): this scenario previously covered the workspace checkpoint summary written
through the memory provider and decision WAL. That feature was removed with memory and compaction; the
scenario now owns the bounded-rebuild contract for account fallback and runtime replacement and was reset
to untested. Earlier verdicts and reports describe the retired behavior and stay as history.
src: .compozy/tasks/memory-removal/adrs/adr-002.md
