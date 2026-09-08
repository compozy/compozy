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
7. Revoke the remembered command shape from the grants section and confirm the next command attempt
   prompts again without affecting shared terminal input.

2026-09-04 targeted re-walk: blocked-verify. The isolated targeted contract required no live provider,
so it could not exercise a fresh agent approval and revocation journey. Runtime catalogs, settings, and
focused approval-grant suites verified the changed contract: `terminal_write` uses ordinary tool policy,
and no terminal-scoped typing prompt, grant row, configuration key, claim, or yield tool remains.
