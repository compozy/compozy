# CH-session-sandbox-first-bind: Bind a logical session's first prompt inside its selected sandbox

```yaml
charter:
  id: CH-session-sandbox-first-bind
  mission: "As Ada, create a logical workspace session and prove its first separately submitted prompt binds exactly once inside the workspace's selected sandbox rather than silently launching unsandboxed."
  mode: strategy-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-15
  scenarios: [RT-session-sandbox-first-bind, RT-session-prompt-idempotency]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Create the workspace-scoped session over CLI and independently read it before any prompt; it must be durable and runtime-unbound without a provider turn."
      - "Submit the first prompt with explicit message and idempotency identities, retry the exact command, and confirm one sandboxed provider turn plus a replay receipt."
      - "Fresh-read session detail and transcript over a second public surface, then stop the session and confirm terminal state without losing the sandbox metadata."
    must_avoid:
      - "Direct database reads, mock providers, omitting the sandbox profile, or treating optimistic command output as the independent confirmation."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in the dated report (Session Debriefs), never here. -->

