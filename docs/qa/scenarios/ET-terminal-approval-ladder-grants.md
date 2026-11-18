---
id: ET-terminal-approval-ladder-grants
area: ET
title: Approve agent terminal commands at the right tier and keep grants revocable
persona: Bruno
journey: J-supervise-agent-terminal
expected: Every agent command shows its exact command and working directory before running, commands that cannot be classified always ask, the fixed irreversible set can never be made automatic, ordinary grants remain listed and revocable, and terminal input has no separate typing grant or administration surface.
entry_points: Pending approvals in the session; terminal exec approval prompt; tool approval grants section; permissions settings allowlist; compozy terminal exec; compozy__terminal_write
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-terminal-shared-control-20260904-204013-041114-lab/qa-artifacts/qa/live-evidence.md; docs/qa/reports/2026-09-04-terminal-shared-control.md
last_report: docs/qa/reports/2026-09-04-terminal-shared-control.md
overlaps: ET-terminal-shared-control; ET-native-tool-approval-grants
---

QA impact 2026-09-04: the terminal-scoped typing grant was deleted. Reset to verify ordinary command
policy still applies and no obsolete typing-grant prompt, row, or configuration survives. Agent terminal
input follows the permission ceiling: `approve-reads` and `approve-all` never prompt for it, while
`deny-all` asks before every `compozy__terminal_write` like any other tool call.

Planned by integrated-terminal task 09 for the approval and grant surface (`_uiux.md` S6). This file
now owns ordinary terminal command policy and the absence of a special typing grant.

Walk:

1. Let an agent run an ordinary command and confirm the approval names the exact command and working
   directory before anything runs; reject one and confirm the agent is told it was rejected.
2. Allowlist a trusted command shape, run it again, and confirm it proceeds without a prompt while the
   remembered shape appears in the grants list.
3. Run a command that cannot be confidently classified — shell indirection, an evaluated string, a
   piped installer — and confirm it prompts even under the autonomous tier.
4. Attempt a command from the fixed irreversible set and confirm it cannot be allowlisted and is
   presented with its destructive treatment.
5. Have an authorized agent write into two terminals and confirm neither path creates a terminal-scoped
   typing prompt or grant; any ordinary native-tool grant behaves like the other tool grants.
6. Confirm no terminal configuration or grants row exposes the removed typing policy.
7. Approve an ordinary `terminal_exec` call with `allow-once` while its prompt remains active and
   confirm the exact command executes once. Repeat the same command and then a changed command;
   both must request a fresh approval, and no remembered grant may appear.
8. If the originating prompt ends or is replaced before dispatch resumes after approval, confirm no
   command runs. The existing `tool_denied` / `session_denied` response must explain the inactive or
   different prompt run. A separate workspace/profile or command-policy denial must still refuse.
9. Revoke the remembered command shape from the grants section and confirm the next command attempt
   prompts again without affecting shared terminal input.

2026-09-04 targeted re-walk: blocked-verify. The isolated targeted contract required no live provider,
so it could not exercise a fresh agent approval and revocation journey. Runtime catalogs, settings, and
focused approval-grant suites verified the changed contract: `terminal_write` uses ordinary tool policy,
and no terminal-scoped typing prompt, grant row, configuration key, claim, or yield tool remains.

2026-11-09 focused verification: the isolated daemon/hosted-MCP approval integration executed three
operator-approved commands (same command twice, then changed arguments), required a fresh `allow-once`
for each call, checked exit status and exact output, and found no persisted grant. The canonical daemon
approval suite deterministically accepted approval before making the originating run inactive or
replacing it, verified zero terminal executions, and checked the public denial message with unchanged
`tool_denied` / `session_denied` classification. This targeted evidence does not re-verify the complete
UI grant/revocation journey above or establish the cause of the historical incident.

Approval correlation follow-up: match each operator decision's request ID to the secret-free
`native tool approval applied` daemon record, then its correlation ID/run/generation to the persisted
`tool.call_completed` outcome. The isolated daemon integration now asserts this join for all three
commands. The canonical dispatch suite also verifies that a conflicting run or generation never
reaches the approval bridge; the daemon approval suite verifies that a later run change still denies.
