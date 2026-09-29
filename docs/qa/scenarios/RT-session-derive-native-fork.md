---
id: RT-session-derive-native-fork
area: RT
title: Forking a session uses the agent's own clone when possible and the carried context otherwise
persona: Rafa
journey: J-15-operate-session-via-cli-api
expected: compozy session fork (POST …/fork, compozy__session_fork) on an idle, bound source whose agent advertises ACP session/fork and session/load (OpenCode) returns seed native_fork with native_state pending and the clone's acp_session_id; the child's first prompt loads the clone and the session read reports native_state loaded while the child answers with the source context; a Claude source returns seed replay; a fork through a message carries that message's turn and nothing after it; a running cut turn is session_turn_in_progress; a clone that cannot load settles failed with native_fork_error and the carried context is sent; the source max_sequence, fences, and log never change and contain no clone-id events.
entry_points: compozy session fork <id> [--message-id <msg>] [--idempotency-key <key>] [-o json]; POST /api/workspaces/{workspace_id}/sessions/{session_id}/fork; GET /api/workspaces/{workspace_id}/sessions/{session_id}/derive/preview[?message_id=…]; compozy session status <child>; compozy session events <source> -o json; compozy__session_fork
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-session-derive-retry, ET-cli-session-continue
---

Planning 2026-09-28 (session-continue-fork task_05): new behavior. Needs a live OpenCode agent (ACP
`session/fork` + `session/load`) and a Claude agent; task_07/08 own the walk.

1. OpenCode source: send one prompt, wait until idle. `GET …/derive/preview` reports
   `native_fork_possible: true`. Note the source `max_sequence` (`compozy session transcript` fences).
2. `compozy session fork <src> -o json`: `derived.seed: native_fork`, `native_state: pending`,
   `acp_session_id` set, `replay_bytes > 0`; `session.derivation.native_state: pending`.
3. Prompt the child ("what did we do so far?"): it answers with the source context.
   `compozy session status <child>` prints `Derivation  seed native_fork · loaded`.
4. Source unchanged: same `max_sequence`; `compozy session events <src> -o json` has no event mentioning
   the clone id.
5. Claude source: `compozy session fork <src> -o json` returns `seed: replay`, no `native_state`, and the
   child's first prompt carries `Context rebuilt from log.` with the source transcript.
6. Fork through a middle user message (`--message-id`): `lineage.origin_message_id` and
   `derived.through_turn_id` are set and the carried context ends with that turn's reply.
7. While the source runs a long turn, fork through that turn's message: `session_turn_in_progress`
   (409); a whole-session fork succeeds with `seed: replay` and `source_turn_in_progress: true`.
8. Clone that cannot load (remove the agent's stored session between fork and first prompt, or use an
   adapter without `session/load` support for that id): the child's first prompt still answers from the
   carried context and the session read shows `native_state: failed` with `native_fork_error`.

Automated evidence at authoring time: `TestForkSession`, `TestForkNativeSeed`, `TestForkNativeGate`,
`TestForkAccountInheritance` (session manager), `TestForkSessionHandler` (HTTP), `TestSessionForkCommand`
(CLI), and `TestDaemonE2ESessionForkCLI` (daemon + acpmock: native pending → loaded, load failure →
failed + carried context, replay-only agent, no clone traffic in the source log).
