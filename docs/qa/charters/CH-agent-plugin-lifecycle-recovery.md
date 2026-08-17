# CH-agent-plugin-lifecycle-recovery: Keep mutations atomic and portable data recoverable

```yaml
charter:
  id: CH-agent-plugin-lifecycle-recovery
  mission: "As Bruno, drive install, enable, update, and remove through competing public planes and prove dotted identity, fixed commit order, crash reconciliation, idempotency, and the portable data contract."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-extension-distribution
  scenarios: [ET-agent-plugin-source-install, ET-agent-plugin-data-removal]
  tour: Interrupt Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Install acme.tools from a local directory and a real immutable git ref, comparing CLI, HTTP, UDS, and native reads of the same persisted instance."
      - "Interrupt before and after registry commit, restart, and retry through a different public plane; fresh reads must show the previous or completed state, never a hybrid or orphan stage."
      - "Prove PLUGIN_DATA is absent before first stdio launch, writable after launch, byte-identical after update, absent after clean remove, and reusable after real delete-to-quarantine fallback."
      - "Force both direct deletion and quarantine rename to fail using the isolated lab filesystem; remove must fail while the extension remains installed and its instance key stays authoritative."
    must_avoid:
      - "Editing the registry or database to manufacture state, running mutations concurrently outside one isolated home, or accepting a completed remove with reachable same-key data."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
