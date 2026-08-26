---
id: RT-subagent-roster-injection
area: RT
title: Carry the described roster inside the call surface itself
persona: Bruno
journey: J-build-a-subagent-roster
expected: An agent definition's description reaches the call tool's parameter, agent list, the agents route and the native tool identically with scope and shadowing, an unknown name fails with the live roster printed inline, and the injected view's caps never bound the registry.
entry_points: .compozy/agents/reviewer/AGENT.md and $COMPOZY_HOME/agents/scout/AGENT.md descriptions; compozy agent list -o json; HTTP and UDS GET /api/agents?workspace=ws_main; compozy__agent_list with {"workspace_id":"ws_main"}; compozy__agent_create with {"name":"reviewer","description":"Reviews diffs","provider":"claude"}; the compozy__agent_call agent parameter descriptor
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-agent-roster-call-compose; RT-agent-call-golden-path; SITE-agent-comms-docs-area
---

The point of the `description` field is that selection costs zero extra turns: the roster travels
inside the call surface, so a caller never has to spend a turn listing agents before delegating.

Author one workspace definition with a description and one without, plus a global definition whose
name collides with the workspace one. Then read the roster from all five surfaces — `compozy agent
list`, `GET /agents`, `compozy__agent_list`, the `compozy__agent_call` parameter description, and the
web catalog — and confirm they agree on name, description, scope, `shadowed` and digest. The
undescribed definition must render its gap rather than have one invented for it, and the shadowed
row must be marked rather than hidden.

Check both caps and prove they bound only the *view*: the injected parameter renders at most 32
definitions at 120 characters of description each, while the full roster stays reachable through
`agent list` and `compozy__agent_list`. Separately, the authoring maximum is 500 characters — a
longer description must fail the load with the bound named, not be silently truncated into the
roster.

Then use it: pick a name straight out of the parameter and confirm the call is accepted with no
lookup turn, and misspell one and confirm `call_agent_unknown` prints the live roster with
descriptions inline and a corrected try line. Finally add a new description while the daemon is
running and confirm it converges without a restart, because rendering happens at serve time.
