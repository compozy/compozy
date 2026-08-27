---
id: SITE-agent-comms-docs-area
area: SITE
title: Teach agent communications with only shipped behavior
persona: Lea
journey: J-build-a-subagent-roster
expected: The agent-comms docs area and the official skill describe calls, mailbox, subagents and budgets exactly as the runtime behaves, its tutorials run end to end against a live daemon, and no spawn vocabulary survives anywhere in docs, CLI reference or API reference.
entry_points: /docs/agent-comms; /docs/agent-comms/calls; /docs/agent-comms/mailbox; /docs/agent-comms/subagents; /docs/agent-comms/budgets-and-safety; /docs/configuration/config-toml; /docs/sessions/orchestration; the generated CLI reference for call and message; the generated API reference for calls and messages; skills/compozy/SKILL.md and references/agent-comms.md
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/scenario-walk-matrix.md
last_report: docs/qa/reports/2026-08-26-agent-comms.md
overlaps: RT-subagent-roster-injection; RT-session-spawn-removed; RT-calls-config-effects
---

Docs are the first surface a newcomer meets, so this walk is a first-time adopter's: read the area
cold and try to succeed from it alone.

Follow each of the five pages' tutorials against a live daemon and confirm the expected outputs
actually match what the runtime prints. Task_07 recorded a known divergence — `_dx.md`'s illustrative
transcripts differ from the shipped human CLI output for await outcomes, list columns and the agents
route, and runtime truth won by operator decision — so the specific job here is spot-verifying the
published transcripts against a running daemon rather than against the spec.

Check the claims the runtime has to back: nine call states, no default deadline, the idle clock
suspending while a call is in flight, no read or seen state, publish being one-way, and the
`[calls]` keys and defaults in the config reference matching what `compozy config get` returns.

Then the hard cut. `sessions/orchestration` must document delegation as calls with the spawn sections
deleted — no aliases and no "formerly known as" — the spawn CLI reference page must be gone with its
verb, and the regenerated API reference must render the Calls and Messages operations rather than an
empty section. Grep the whole docs tree and `skills/compozy/` for spawn vocabulary and confirm it is
clean. Finally confirm navigation, icons and the parent `meta.json` place the area correctly and no
link is broken.
