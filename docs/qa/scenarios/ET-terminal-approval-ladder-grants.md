---
id: ET-terminal-approval-ladder-grants
area: ET
title: Approve agent terminal work at the right tier and keep the grants revocable
persona: Bruno
journey: J-supervise-agent-terminal
expected: Every agent command shows its exact command and working directory before running, commands that cannot be classified always ask, the fixed irreversible set can never be made automatic, typing is granted per terminal and never by policy, and every grant is listed and revocable where the other tool grants live.
entry_points: Pending approvals in the session; terminal exec approval prompt; typing-grant prompt; tool approval grants section; permissions settings allowlist; compozy terminal exec
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/test-e2e-runtime-after-fix.log; docs/qa/reports/2026-08-26-integrated-terminal.md
last_report: docs/qa/reports/2026-08-26-integrated-terminal.md
overlaps: ET-terminal-agent-handoff-input; ET-native-tool-approval-grants
---

Planned by integrated-terminal task 09 for the approval and grant surface (`_uiux.md` S6), which no
task 06–08 scenario owned. `ET-terminal-agent-handoff-input` exercises typing under an already-granted
terminal; this file owns how the grant is asked for, refused, remembered, and revoked. Task 10 owns
the walk, evidence, and verdict.

Walk:

1. Let an agent run an ordinary command and confirm the approval names the exact command and working
   directory before anything runs; reject one and confirm the agent is told it was rejected.
2. Allowlist a trusted command shape, run it again, and confirm it proceeds without a prompt while the
   remembered shape appears in the grants list.
3. Run a command that cannot be confidently classified — shell indirection, an evaluated string, a
   piped installer — and confirm it prompts even under the autonomous tier.
4. Attempt a command from the fixed irreversible set and confirm it cannot be allowlisted and is
   presented with its destructive treatment.
5. Have the agent type into a fresh terminal for the first time and confirm the human is asked once;
   reject on one terminal and allow on another, then confirm follow-up typing on the allowed terminal
   does not prompt while the other terminal asks again.
6. Confirm no configuration makes typing on a fresh terminal automatic.
7. Revoke the typing grant and the remembered command shape from the grants section and confirm the
   next attempt prompts again.
