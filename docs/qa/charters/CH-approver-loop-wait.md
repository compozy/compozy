# CH-approver-loop-wait: Decide one durable approval without a ghost or duplicate outcome

```yaml
charter:
  id: CH-approver-loop-wait
  mission: "As Marina on 4G, open the exact approval link, decide once despite a network interruption, and prove the same durable wait resumes or halts across every public surface."
  mode: charter-with-tour
  persona:
    name: Marina
    device: phone-large
    network: 4g
    locale: en-US
  journey: J-03
  scenarios: [LP-approval-link-journey]
  tour: Network Tour
  time_box_minutes: 60
  surfaces: [Web, CLI, HTTP, UDS, SSE]
  browser_plan: "Use browser-use:browser (Playwright-backed) with a phone-large viewport and network throttling; if unavailable, use agent-browser and record any missing throttle support."
  automated_precondition: "make test-e2e-runtime passes against the final build before the session starts."
  cross_surface_plan: "After the browser decision, compare structured CLI JSON with the public HTTP and UDS wait/run payloads, then reload the approval URL and run detail to prove the same winner and terminal state."
  evidence_expectations: [approval-link screenshot, throttled-request evidence, decision screenshot, CLI JSON transcript, HTTP and UDS response bodies, reload confirmation]
  guidance:
    must_try:
      - "Open the exact approval URL and confirm it resolves one active wait; a stale or ambiguous link must fail visibly."
      - "Drop the network mid-decision, restore it, and race a duplicate CLI or HTTP decision; exactly one winner may resume or halt the node."
      - "Advance one authored escalation step before deciding and prove the accepted decision cancels every later step."
      - "Approve, request changes, and reject on separate seeded runs; each outcome remains truthful after refresh."
    must_avoid:
      - "Self-approval by the owning agent, database reads, or a desktop-only substitute for the phone journey."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
