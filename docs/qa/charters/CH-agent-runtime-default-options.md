# CH-agent-runtime-default-options: Author reusable runtime defaults across every public surface

```yaml
charter:
  id: CH-agent-runtime-default-options
  mission: "As Ada, author one agent with Reasoning, Fast, and typed ACP-option defaults through every supported structured create surface, then prove fresh reads and sessions preserve the same logical values."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-18
  scenarios: [MS-web-agent-create-simple-advanced, RT-070]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Create equivalent workspace agents through Web, HTTP, UDS, CLI, and compozy__agent_create with Fast plus one select and one boolean ACP option."
      - "Fresh-read each definition through Web, HTTP, UDS, CLI, and AGENT.md; compare logical provider/model values and every typed default without exposing a private transport alias."
      - "Start a session without overrides, then with durable-session and prompt overrides; prove precedence is resolved per field and per option ID."
      - "Change provider and model during authoring and prove unsupported defaults are cleared before persistence."
    must_avoid:
      - "Do not claim a native agent read or update tool exists; use the real read surfaces."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report. -->
