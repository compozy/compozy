# CH-agent-plugin-diagnostics-parity: Make degradation visible and recoverable everywhere

```yaml
charter:
  id: CH-agent-plugin-diagnostics-parity
  mission: "As Ada, inspect warning-only, fully degraded, and live MCP failure states across every read plane, then recover them without hiding skips or poisoning healthy siblings."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: flaky
    locale: en-US
  journey: J-extension-kit-lifecycle
  scenarios: [ET-agent-plugin-degraded-inventory]
  tour: Network Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Install the mixed and fully degraded fixtures; compare ordered format and diagnostics across CLI human/json/jsonl/toon, HTTP, UDS, native info/list/inventory, and the installed Web detail."
      - "Cause a PATH-missing stdio failure and remote auth/connection failure; live extension_mcp_server_unhealthy rows follow immutable ingest skips and affect only the named servers."
      - "Restore each dependency and perform a successful exchange; its live diagnostic clears, older failures cannot overwrite success, and reload/update/disable/remove evict only the affected generation."
      - "Restart the daemon and prove live health starts unknown while persisted ingest diagnostics remain; capture the known CLI parity bug separately if reproduced."
    must_avoid:
      - "Reading daemon internals as the verdict, treating zero usable components as a native empty extension, or accepting copy-only agreement when codes, scope, ordering, or freshness differ."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
