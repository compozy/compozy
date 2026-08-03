# CH-operator-loop-recovery: Keep healthy work moving while repairing one Loop lane

```yaml
charter:
  id: CH-operator-loop-recovery
  mission: "As Bruno, interrupt and degrade live Loop nodes, then use only declared lifecycle controls to preserve healthy work, repair the affected lane, and reach a truthful finish."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-recover-loop-node-failure
  scenarios: [LP-sick-target-degrades-one-lane, LP-quarantine-diagnose-requeue, LP-crash-death-resume, LP-cancel-vs-kill, LP-016, TA-084, LP-days-long-node-no-clock, LP-live-pause-repair-resume, LP-operator-lifecycle-ui]
  tour: Interrupt Tour
  time_box_minutes: 90
  surfaces: [Web, CLI, HTTP, UDS, SSE]
  browser_plan: "Drive run detail and inventories with browser-use:browser (Playwright-backed); if unavailable, restart with agent-browser and disclose the fallback."
  automated_precondition: "make test-e2e-runtime passes against the final build before the session starts."
  cross_surface_plan: "At every pause, death, quarantine, requeue, cancel, and kill boundary, capture structured CLI JSON and the matching public HTTP response, then reload the Web route and compare state, cause, provenance, and allowed actions."
  evidence_expectations: [state-transition screenshots, CLI JSON transcript, HTTP and UDS response bodies, SSE winner event, daemon-restart checkpoint, fresh-load confirmation]
  guidance:
    must_try:
      - "Open one target-scoped breaker while a healthy lane continues, inspect the resulting quarantine, repair the target, and requeue with actor provenance."
      - "Pause and resume at a safe boundary, kill a checkpointing managed session, restart the daemon during durable work, and prove one fenced continuation."
      - "Advance the isolated clock across days without an authored timeout; silence may raise attention but must not stop the node, and later evidence must clear it."
      - "Cancel one run cooperatively and kill another immediately, repeat incompatible verbs, and prove every retired stop surface is absent."
    must_avoid:
      - "Reading storage or coordinator memory as proof, manually inventing allowed actions, or treating automated integration output as the session verdict."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
