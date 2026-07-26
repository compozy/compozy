# CH-live-bounds-agent-path: Interrupt bounded Live collaboration through structured surfaces

```yaml
charter:
  id: CH-live-bounds-agent-path
  mission: "As Ada, drive one explicit Live execution through CLI, HTTP, UDS, and native tools, interrupt it at its admission and execution boundaries, and prove every durable message, wake, stop reason, and usage total remains bounded and workspace-isolated."
  mode: strategy-based
  persona:
    name: Ada
    device: desktop
    network: flaky
    locale: en-US
  journey: J-run-bounded-live-collaboration
  scenarios: [NB-run-bounded-live-collaboration, NB-agent-manages-participation, NB-020]
  tour: Interrupt Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Bootstrap a fresh isolated lab with unique AGH_HOME/ports/provider home/tmux socket, register every daemon/provider process, and execute eval \"$TEARDOWN_COMMAND\" (or make qa-reap) on every terminal path; cite teardown.json clean=true."
      - "Start a Local control and an explicit Live owner with small finite bounds. Send an eligible direct, an eligible mention, ineligible control kinds, a same-root burst, a distinct root, and a depth-capped reply; compare durable dispositions with provider activation count."
      - "Interrupt once before claim by restarting the daemon, once during provider work by disabling Network/canceling, and once at exhaustion. Re-read each source, task-run wake, terminal settlement, conversation, and per-run/channel/workspace usage; no source may activate twice."
      - "Compare CLI -o json, HTTP, UDS, native tools, and agent context for mode/source/channel/bounds/consumption/usage plus network_participation_unavailable, not_participating, loop_requires_live, unknown-channel, unsupported, authority, and invalid-combination diagnostics."
    must_avoid:
      - "Using a second prompt to unstick the agent, treating the existing real-agent no-prompt completion failure as fixed by this charter, inspecting internal tables as the primary verdict, or expanding into mailbox/spend caps."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief belongs in the dated report. -->
