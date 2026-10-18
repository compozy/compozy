---
id: RT-session-derive-native-fork
area: RT
title: Forking a session uses the agent's own clone when possible and the carried context otherwise
persona: Rafa
journey: J-15-operate-session-via-cli-api
expected: compozy session fork (POST …/fork, compozy__session_fork) on an idle, bound source whose agent advertises ACP session/fork and session/load (runtime.acp_caps.supports_fork_session and supports_load_session true; today the real opencode, claude, and codex adapters all do) returns seed native_fork with native_state pending and the clone's acp_session_id; the child's first prompt loads the clone and the session read reports native_state loaded while the child answers with the source context; a source that is not eligible (agent without the fork capability, unbound or stopped source) returns seed replay; a fork through a message carries that message's turn and nothing after it; a running cut turn is session_turn_in_progress; a clone that cannot load (for example Codex while its source process still holds the conversation) settles failed with native_fork_error and the carried context is sent; the source max_sequence, fences, and log never change and contain no clone-id events.
entry_points: compozy session fork <id> [--message-id <msg>] [--idempotency-key <key>] [-o json]; POST /api/workspaces/{workspace_id}/sessions/{session_id}/fork; GET /api/workspaces/{workspace_id}/sessions/{session_id}/derive/preview[?message_id=…]; compozy session status <child>; compozy session events <source> -o json; compozy__session_fork
qa_status: pass
bug_ids: BUG-20260928-native-fork-clone-load-refused
fix_status: fixed
retest_status: pass
fix_commits: uncommitted (task_08 part B1)
evidence: docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/native-fork.txt; docs/qa/evidence/2026-09-28-session-continue-fork-b1/oc-fork.json; docs/qa/evidence/2026-09-28-session-continue-fork-b1/oc-fork-p1.txt; docs/qa/evidence/2026-09-28-session-continue-fork-b1/cl-fork.json; docs/qa/evidence/2026-09-28-session-continue-fork-b1/cx-fork3.json; docs/qa/evidence/2026-09-28-session-continue-fork-b1/cx-fork3-p1.txt; docs/qa/evidence/2026-09-28-session-continue-fork-b1/cx-fork-msg.json; docs/qa/evidence/2026-09-28-session-continue-fork-b1/long-fork-running-cut.txt; docs/qa/evidence/2026-09-28-session-continue-fork-b1/codex-acp-fork-close-probe.txt
last_report: docs/qa/reports/2026-09-29-session-continue-fork-r1-rewalk.md
overlaps: RT-session-derive-retry; ET-cli-session-continue
---

Planning 2026-09-28 (session-continue-fork task_05): new behavior. Needs a live agent that advertises
ACP `session/fork` + `session/load` and a replay-only source (non-advertising agent or ineligible source);
task_07/08 own the walk.

Capability-based: pick sources by what the agent advertises (`GET /api/sessions/<src>` →
`session.runtime.acp_caps.supports_fork_session` and `supports_load_session`), not by provider name. As of
2026-09-28 all three real adapters (opencode, claude, codex) advertise fork and load; Codex refuses to
load the clone while the source process lives, which exercises step 8.

1. Source whose agent advertises fork and load: send one prompt, wait until idle. `GET …/derive/preview`
   reports `native_fork_possible: true`. Note the source `max_sequence` (`compozy session transcript` fences).
2. `compozy session fork <src> -o json`: `derived.seed: native_fork`, `native_state: pending`,
   `acp_session_id` set, `replay_bytes > 0`; `session.derivation.native_state: pending`.
3. Prompt the child ("what did we do so far?"): it answers with the source context.
   `compozy session status <child>` prints `Derivation  seed native_fork · loaded`.
4. Source unchanged: same `max_sequence`; `compozy session events <src> -o json` has no event mentioning
   the clone id.
5. Source that is not native-eligible (an agent without `supports_fork_session`, such as the acpmock
   replay-only agent, or a stopped/unbound source whose preview reports `native_fork_possible: false`):
   `compozy session fork <src> -o json` returns `seed: replay`, no `native_state`, and the child's first
   prompt carries `Context rebuilt from log.` with the source transcript.
6. Fork through a middle user message (`--message-id`): `lineage.origin_message_id` and
   `derived.through_turn_id` are set and the carried context ends with that turn's reply.
7. While the source runs a long turn, fork through that turn's message: `session_turn_in_progress`
   (409); a whole-session fork succeeds with `seed: replay` and `source_turn_in_progress: true`.
8. Clone that cannot load (a Codex source whose process is still alive, removing the agent's stored
   session between fork and first prompt, or an adapter without `session/load` support for that id): the
   child's first prompt still answers from the carried context and the session read shows
   `native_state: failed` with `native_fork_error`.

Automated evidence at authoring time: `TestForkSession`, `TestForkNativeSeed`, `TestForkNativeGate`,
`TestForkAccountInheritance` (session manager), `TestForkSessionHandler` (HTTP), `TestSessionForkCommand`
(CLI), and `TestDaemonE2ESessionForkCLI` (daemon + acpmock: native pending → loaded, load failure →
failed + carried context, replay-only agent, no clone traffic in the source log).

Planning 2026-09-28 (session-continue-fork task_07): task_08 runs steps 1–4 and 8 on the real
`opencode` binary (`opencode acp`; also confirms `runtime.acp_caps.supports_fork_session: true`, the
task_02 deferral) and step 5 on the real `claude` binary, plus a Codex source as the second replay
agent. acpmock `session_fork_fixture.json` (`fork-native-agent`, `fork-load-missing-agent`) is the
fallback for step 8 when the real adapter cannot be made to lose its stored session. Plan: `docs/qa/reports/2026-09-28-session-continue-fork-plan.md`.

## 2026-09-28 walk (task_08 part B1) — FIXED

Rafa, Money Tour, isolated lab `compozy-session-continue-fork-b1-20260929-010817-219689-lab`, real `opencode` 1.18.33, `claude` (claude-agent-acp), `codex` (codex-acp 2.0.0).
All three real adapters advertise `supports_fork_session`, `supports_load_session`, and `supports_resume_session: true`, so step 5's
"a Claude source returns seed replay" no longer holds for today's adapters: Claude and Codex idle sources fork natively too. The
replay path was walked on a message cut (step 6), a running source (step 7), and a stopped source (preview `native_fork_possible: false`).
1–4 OpenCode: preview `native_fork_possible: true`; fork → `native_fork`/`pending`/clone `ses_…`; the child answered `HERON-7`;
status `seed native_fork · loaded`; OpenCode's own store shows the clone holding the source turn and the child prompt without
`Context rebuilt`; source `max_sequence 7`, meta sha, runtime unchanged; no source event mentions the clone id.
Claude native fork: loaded, child answered `PELICAN-42`, the clone's next prompt carries no replay.
Codex native fork: FAILED — the child could not load the clone while the source process lived (`thread … already has an active
writer`) and stayed pending → BUG-20260928-native-fork-clone-load-refused, fixed (clone load refusal settles `failed` and replays).
Re-walk: child answered `PELICAN-42`, status `seed native_fork · failed: session/load: internal error (replayed the carried context)` (also step 8 on a real adapter).
6. Fork through message 2 of 4 (Codex): `origin_message_id` + `through_turn_id` set, `seed replay`, 4 of 8 messages; the child quoted
the second question, not the later ones. 7. acpmock `long-agent` mid-turn: fork through the running message → `session_turn_in_progress`
(CLI exit 65, HTTP 409); whole fork → `seed replay`, `source_turn_in_progress: true`; the source kept running until cancel.

## 2026-09-29 spot check (review round 1) — PASS

Capability-based, acpmock. `fork-native-agent` source: `GET /api/sessions/<id>` → `session.runtime.acp_caps.supports_fork_session: true`, `supports_load_session: true`. Preview `native_fork_possible: true`; `session fork -o json` → `seed native_fork`, `native_state pending`, clone `fork-native-agent-fork-2`. The child's first prompt printed `Derivation seed native_fork · loaded`, and no source event mentions a clone. `alpha` source (`supports_fork_session: false`) → `seed replay`, no `native_state`. The new snapshot revalidation under the prompt slot kept an idle source native. The advance-between-snapshot-and-slot race stays owned by `TestForkNativeGate`. Wording fix: `acp_caps` is on `GET /api/sessions/<id>` (`session.runtime.acp_caps`), not on `compozy session status -o json`, which has no `runtime` block. Evidence: `docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/native-fork.txt`.
