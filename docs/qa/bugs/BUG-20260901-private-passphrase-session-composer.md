# BUG-20260901-private-passphrase-session-composer: Agent asks for a private passphrase in visible chat

- **Status:** fixed
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Marina
- **Journey Step:** J-supervise-agent-terminal, hidden input handoff
- **Scenarios:** ET-terminal-agent-handoff-input; ET-terminal-redaction-boundaries
- **Found:** 2026-09-01 · **Report:** docs/qa/reports/2026-09-01-integrated-terminal-review-r2.md

## Summary

After completing a visible terminal command, the agent asked Marina to enter a private passphrase in
the ordinary session composer. That surface persists its input in the conversation transcript, so the
request contradicted the promise that the value would remain private.

## Reproduction

- **Charter:** CH-terminal-lease-fencing-takeover · **Tour:** Interrupt Tour
- **Environment:** desktop / wifi-fast / en-US / isolated local lab

1. Start a managed `general` session and ask it to open a project terminal, run a readiness command,
   then request a short private passphrase without including the value in its reply.
2. Wait for the agent to complete the terminal command.
3. Inspect where the agent asks for the passphrase.

**Expected:** The agent keeps a terminal running and creates a redacted
`compozy__terminal_request_input` handoff.
**Actual:** The agent asks for the passphrase in the ordinary session composer.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-review-r2-20260902-020216-937662-lab/qa-artifacts/qa/screenshots/marina-private-passphrase-in-session-composer.png`
- Session `sess-b07c6b4e6f291093`, terminal `term-14418a710470`; public session list shows the session idle
  with no pending interaction after the request.

## Fix

- **Root cause:** The official terminal reference limited `terminal_request_input` to a program waiting
  for input and did not state that private values requested by the agent's own terminal workflow must
  use the same redacted surface.
- **Fix commit:** pending remediation batch
- **Regression test:** documented fresh-agent replay; automated stochastic skill-fidelity coverage is
  tracked in `docs/qa/automation-backlog/terminal-private-input-skill-fidelity.md`.

## Verification

- **Retested:** 2026-09-02 in fresh session `sess-0dd350aef0908b61`.
- **Result:** Passed. The agent kept terminal `term-b264e7c1d26a` visible, used the protected input
  request surface, and exercised both answer and decline without moving the value into session chat.
