---
id: ET-cli-session-continue
area: ET
title: Continue a session with another agent from the CLI
persona: Rafa
journey: J-15-operate-session-via-cli-api
expected: compozy session continue <id> --agent <name> [--message …] creates exactly one new user session in the source's workspace with lineage.kind continue, prints the Golden Path block (Continued … into …, Origin, Context N messages · X KiB · nothing omitted, Seed replay, First prompt admitted|staged), -o json matches the API SessionDeriveResponse, the child's first prompt carries the source conversation as "Context rebuilt from log." + <compozy_context_replay> ahead of "User request:", the source session's max_sequence and meta are unchanged, usage errors exit 2 with the documented messages, and an unknown agent prints agent_not_found.
entry_points: compozy session continue <id> --agent <name> [--provider/--model/--reasoning-effort/--speed/--acp-option | --route <n>] [--name] [--message] [--expected-epoch --expected-generation --expected-max-sequence] [--idempotency-key] [-o json]; POST /api/workspaces/{workspace_id}/sessions/{session_id}/continue; GET …/derive/preview; compozy session status <child>; compozy logs --session <child> --type session.derived -o json; compozy__session_continue
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-session-derive-retry; RT-provider-error-handoff
---

Planning 2026-09-28 (session-continue-fork task_03): new behavior. Walk against a dev daemon with two
agents (acpmock or real Codex + Claude):

1. Create a Codex session and run two prompts. Record `compozy session status <src> -o json` and
   `GET …/transcript` fences (`epoch`, `generation`, `max_sequence`).
2. `GET /api/workspaces/<ws>/sessions/<src>/derive/preview` reports `message_count`, `replay_bytes`,
   `truncated: false`, `native_fork_possible`, and the same fences.
3. `compozy session continue <src> --agent claude-code --message "Carry on; run the tests first."`
   prints the Golden Path block with `First prompt  admitted`.
4. The Claude session's first prompt (agent log or acpmock record) starts with `Context rebuilt from
   log.`, names the source and origin agent, contains the Codex conversation inside
   `<compozy_context_replay>`, and ends with `User request:` + the message.
5. `compozy session status <child>` prints `Origin  continue · from <src> · codex` and
   `Derivation  seed replay · first prompt admitted`; `-o json` has `lineage.kind: "continue"` and
   `derivation`.
6. The source's `max_sequence`, `epoch`, `generation`, runtime, and `meta.json` bytes are unchanged.
7. `compozy logs --session <child> --type session.derived -o json` shows one event.
8. Usage errors exit 2: no `--agent` (`cli: --agent is required`), `--route 2 --speed fast` (route/runtime
   rule), `--expected-epoch 3` alone (fence rule). `--agent nope` prints `agent_not_found`.

Automated evidence at authoring time: CLI command tests (`TestSessionContinueCommand`), HTTP transport
tests (`TestContinueSessionHandler`, `TestPreviewSessionDeriveHandler`), the native tool binding case in
`TestDaemonNativeTools`, and the session manager derive suites. task_07/08 own the walk.
