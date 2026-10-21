---
id: ET-terminal-session-block-handoff
area: ET
title: Follow a deliberate terminal run from the session transcript
persona: Bruno
journey: J-operate-integrated-terminal
expected: A deliberate terminal tool call renders a live block in the transcript with its exit outcome, jumps to the owning terminal window, hands a still-running command over instead of pretending it finished, and marks the output it feeds back to the model as untrusted data.
entry_points: Session transcript terminal block; Open terminal affordance; Web dock Terminal app; compozy__terminal_exec with visible true; compozy__terminal_read; compozy__terminal_wait
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-terminal-shared-control-20260904-204013-041114-lab/qa-artifacts/qa/live-evidence.md; docs/qa/reports/2026-09-04-terminal-shared-control.md
last_report: docs/qa/reports/2026-09-04-terminal-shared-control.md
overlaps: ET-terminal-browser-lifecycle; ET-agent-terminal-window-materialization
---

QA impact 2026-09-04: supervised terminal blocks no longer carry controller or watch state. Reset to
verify live output, window focus, presence, and untrusted-data marking under shared control.

Planned by integrated-terminal task 09 for the deliberate-use transcript surface (`_uiux.md` S3);
this file owns the supervised block and its handoff. Reset by ADR-019 (S4 removed; agent-opened
terminals materialize windows) and re-walked on 2026-09-01 — see `last_report`.

Walk:

1. Ask an agent to run a short visible command in a terminal and watch the transcript block stream its
   output while the command is running.
2. Let the command finish and confirm the block shows the exit outcome, including a non-zero exit
   rendered as a plain code rather than an alarm.
3. Use the block's open affordance and confirm it focuses the terminal window the daemon already
   materialized for this run rather than opening a second one.
4. Start a command that outlives the yield window and confirm the transcript hands over the running
   terminal's identity instead of claiming completion.
5. Read the same output back through the agent's read and wait paths and confirm every model-facing
   result carries the untrusted-data marking.
6. Confirm the supervised block carries truthful viewer presence without a controller identity, and
   that agent-internal commands in the same transcript render as plain command output with no terminal
   chrome.

2026-09-04 targeted re-walk: blocked-verify. The isolated targeted contract required no live provider,
so it could not create a fresh agent transcript block. The browser terminal, public projections, and
focused component suites verified the changed invariant: viewer presence has no controller identity and
terminal output remains untrusted. A future provider-backed scenario should re-walk the complete block.

## Declared launch environment (#828)

In an isolated home, declare a disposable Vault token and optional `env:SSH_AUTH_SOCK` on the
workspace's default agent. Use a disposable socket path, never the operator's authentication socket.

1. Launch a human terminal and an agent terminal command. Verify each process can use its declared
   token and socket without printing the token. An undeclared credential remains filtered.
2. Remove the optional source and launch again: the target must be absent, including when a stale
   caller override exists. Remove a required source: launch must fail before process creation.
3. Give the agent a different profile and verify foreign profile Vault references fail. A stale
   run or runtime generation must not obtain declarations from the replacement session.
4. Delegate a worker with no account override, then one with its own `session_env`. Verify the first
   inherits the creator account and the second uses its explicit declaration.
5. Change a declared account reference before resume. Verify context replay occurs rather than
   loading the previous account's native session. Inspect diagnostics and retained command artifacts
   for the disposable secret; only redacted content may appear.

Focused validation covers real Unix pipe environment injection/removal, terminal exec lifetime,
session authority, worker precedence and resume affinity. The full provider-backed UI walk remains pending.

2026-10-20 targeted #828 re-walk: the isolated daemon's human `terminal exec` path passed env/Vault
injection, disposable socket injection, undeclared filtering, stale optional override removal,
required-missing pre-launch failure, and output redaction. CLI/API/runtime behavior passed;
manifest-owned teardown was clean. Agent transcript and Loop inheritance remain covered by focused
runtime suites, with a provider-backed UI walk still unverified. See the issues verification report.

## Console lifecycle regression (#873)

Run long commands with and without visibility. Only the catalog-confirmed running PTY streams in the transcript; pipe and removed terminal blocks keep recorded output without socket retries. A direct pipe attach-ticket request returns terminal_not_interactive. Markdown host image paths render as references without app-origin requests. Terminal manager, real HTTP/SQLite/PTY/WebSocket lifecycle and component regressions pass; a provider-backed transcript walk remains unverified.

Targeted browser evidence (2026-10-21): the isolated Web Storybook rendered
`MessageMarkdown/HostImageReference` at 1920×963 with readable authored path, zero image elements,
and zero resource requests for that host path. `SessionTerminalBlock/StillRunningHandoff` retained
the output, running label and Open terminal action with no attach-ticket request. This is rendered
component evidence, not a provider-backed session lifecycle claim.
