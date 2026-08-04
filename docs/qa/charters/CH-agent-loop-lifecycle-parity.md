# CH-agent-loop-lifecycle-parity: Manage lifecycle and durable waits through structured surfaces

```yaml
charter:
  id: CH-agent-loop-lifecycle-parity
  mission: "As Ada, manage Loop nodes and durable waits entirely through native tools, then prove deterministic CLI, HTTP, and UDS parity across restart, duplicate delivery, and workspace boundaries."
  mode: strategy-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-07
  scenarios: [LP-agent-operates-lifecycle-via-native-tools, TA-070, TA-076, LP-durable-wait-restart, LP-waiting-inventory-escalation, LP-duplicate-event-suppressed]
  tour: Feature Tour
  time_box_minutes: 90
  surfaces: [native-tools, CLI, HTTP, UDS, SSE]
  browser_plan: "No browser interaction belongs to Ada's headless journey; Web parity is settled by the author, operator, and approver charters with browser-use:browser or agent-browser fallback."
  automated_precondition: "make test-e2e-runtime passes against the final build before the session starts."
  cross_surface_plan: "Diff every native-tool response against structured CLI JSON and the public HTTP/UDS payload for the same workspace and identity; use a fresh read after daemon restart and after every losing race."
  evidence_expectations: [native-tool transcript, CLI JSON transcript, HTTP and UDS response bodies, SSE event excerpt, restart checkpoint, workspace-isolation comparison]
  guidance:
    must_try:
      - "Discover exactly eight lifecycle tools, invoke every run and node action, repeat invalid losers, and confirm compozy__loop_stop is unknown."
      - "Create timer, event, and approval waits, restart the daemon, and prove each identity and escalation step survives and resumes once."
      - "Reject a watch response without event_key, redeliver one valid key across restart, and confirm loud suppression until the configured horizon expires."
      - "List and mutate from workspace A, then prove an A-scoped agent cannot see or act on workspace B nodes."
    must_avoid:
      - "Web UI, TTY parsing, SQLite reads, internal Go calls, or reconstructed manifest values."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
