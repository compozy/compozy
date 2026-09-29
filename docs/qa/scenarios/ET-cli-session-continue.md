---
id: ET-cli-session-continue
area: ET
title: Continue a session with another agent from the CLI
persona: Rafa
journey: J-15-operate-session-via-cli-api
expected: compozy session continue <id> --agent <name> [--message …] creates exactly one new user session in the source's workspace with lineage.kind continue, prints the Golden Path block (Continued … into …, Origin, Context N messages · X KiB · nothing omitted, Seed replay, First prompt admitted|staged), -o json matches the API SessionDeriveResponse, the child's first prompt carries the source conversation as "Context rebuilt from log." + <compozy_context_replay> ahead of "User request:", the source session's max_sequence and meta are unchanged, usage errors exit 2 with the documented messages, and an unknown agent prints agent_not_found.
entry_points: compozy session continue <id> --agent <name> [--provider/--model/--reasoning-effort/--speed/--acp-option | --route <n>] [--name] [--message] [--expected-epoch --expected-generation --expected-max-sequence] [--idempotency-key] [-o json]; POST /api/workspaces/{workspace_id}/sessions/{session_id}/continue; GET …/derive/preview; compozy session status <child>; compozy logs --session <child> --type session.derived -o json; compozy__session_continue
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/cli-errors.txt; docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/cli-committed-child.txt; docs/qa/evidence/2026-09-28-session-continue-fork-b1/cx-continue-cl.txt; docs/qa/evidence/2026-09-28-session-continue-fork-b1/cl-child-transcript.json; docs/qa/evidence/2026-09-28-session-continue-fork-b1/cl-child-derived-logs.json; docs/qa/evidence/2026-09-28-session-continue-fork-b1/cx-continue-cli.json; docs/qa/evidence/2026-09-28-session-continue-fork-b1/cx-continue-api.json; docs/qa/evidence/2026-09-28-session-continue-fork-b1/cx-continue-usage-errors.txt
last_report: docs/qa/reports/2026-09-29-session-continue-fork-r1-rewalk.md
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
   rule), `--expected-epoch 3` alone (fence rule). `--agent nope` prints
   `Error: agent_not_found: no agent named "nope"` and exits 1 (daemon errors print `<code>: <message>`).

Automated evidence at authoring time: CLI command tests (`TestSessionContinueCommand`), HTTP transport
tests (`TestContinueSessionHandler`, `TestPreviewSessionDeriveHandler`), the native tool binding case in
`TestDaemonNativeTools`, and the session manager derive suites. task_07/08 own the walk.

## 2026-09-28 walk (task_08 part B1) — PASS

Rafa, Money Tour, isolated lab `compozy-session-continue-fork-b1-20260929-010817-219689-lab`, real Codex (codex-acp, `gpt-5.6-luna`) → real Claude (claude-agent-acp, `claude-haiku-4-5-20251001`).
1–2. Codex source `sess-b585138e835a4e38`, two turns (codeword `PELICAN-42`, `2+3`); preview `message_count 4`, `replay_bytes 681`,
`truncated false`, fences `0/0/17`. 3. `session continue --agent cl-agent --message …` printed the Golden Path block with
`First prompt  admitted`. 4. Independent read in Claude's own session log: the first user message is `User request:` →
`Context rebuilt from log.` (names the source and `cx-agent`) → `<compozy_context_replay>` with the 4 Codex messages →
`User request:` + situation + the message; the child answered `PELICAN-42`. 5. `session status <child>` prints `Origin continue ·
from … · cx-agent` and `Derivation seed replay · first prompt admitted`; `GET /api/sessions/<child>` has `lineage.kind continue` +
`derivation`. 6. Source `max_sequence`, epoch, generation, runtime, and `meta.json` sha unchanged. 7. One `session.derived`.
8. Usage errors exit 2 with the documented messages; `--agent nope` exits 69 (`no agent named "nope"`, `-o json` code
`agent_not_found`). CLI `-o json` and `POST …/continue` have identical field sets.
Paper cuts: the human output of an unknown agent does not print the `agent_not_found` code; `compozy logs --session <id>` needs a
registered cwd or `--workspace` although the session id identifies the workspace. Native-tool parity not re-walked (automated).

## 2026-09-29 re-walk (review round 1) — PASS

Rafa, isolated lab `compozy-session-continue-fork-r1-rewalk-20260929-041438-936027-lab`, branch head `62bdd8054`, acpmock. Golden Path re-checked (`alpha` → `beta` with `--message`: `First prompt admitted`). Step 8: `--agent` missing, `--route 2 --speed fast`, and a partial fence exit 2 with the documented messages. `--agent nope` prints `error: agent_not_found: no agent named "nope"` and exits 1; `-o json` has `code: agent_not_found`. A missing source prints `error: session_not_found: …` and exits 1. The CLI renderer prefixes errors with lowercase `error:`; `_dx.md` examples show `Error:`, which is illustrative only. Post-commit failure (`bad-model`, `--message`): exit 1, the daemon error, then `session sess-… was already created; open it with `compozy session status sess-…`, or rerun with --idempotency-key qa-r1-committed to get it back`. `-o json` carries `child_session_id`, and the same key reuses that one child. Paper cut: the 422 has no top-level `code` (only `diagnostic.code: model_unavailable`), so the human line has no `<code>:` prefix and shows the raw internal chain. Evidence: `docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/cli-errors.txt`, `docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/cli-committed-child.txt`, `docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/cli-continue.txt`.
