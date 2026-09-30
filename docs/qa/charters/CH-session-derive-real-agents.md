# CH-session-derive-real-agents: Continue and fork across real agents from the CLI and API

```yaml
charter:
  id: CH-session-derive-real-agents
  mission: "As Rafa, continue and fork real OpenCode, Claude, and Codex sessions from the CLI and HTTP, and prove the child receives the source conversation (native clone on OpenCode, carried context on Claude/Codex) while the source never changes."
  mode: charter-with-tour
  persona:
    name: Rafa
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-15-operate-session-via-cli-api
  scenarios: [ET-cli-session-continue, RT-session-derive-native-fork]
  tour: Money Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Codex source → `session continue --agent <claude agent> --message …`: Golden Path block, `First prompt  admitted`, the child answers from the carried context; `-o json` field-diffs against POST …/continue."
      - "OpenCode source idle and bound: `runtime.acp_caps.supports_fork_session: true`, preview `native_fork_possible: true`, `session fork` → `native_fork` pending → loaded at the child's first prompt, which answers with the source context."
      - "Claude and Codex sources: `session fork` returns `seed: replay`; `--message-id` through a middle message carries that turn and nothing after it."
      - "Source invariants after every derive: same `max_sequence`, `epoch`, `generation`, runtime, and no event mentioning a clone id."
    must_avoid:
      - "Spending provider quota on long prompts — one or two short turns per source are enough."
      - "Treating an acpmock pass as proof of the real-adapter native path; acpmock is only the fallback for the load-failure leg."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
