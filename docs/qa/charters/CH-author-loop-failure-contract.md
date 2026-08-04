# CH-author-loop-failure-contract: Author a failure contract and prove the run honors it

```yaml
charter:
  id: CH-author-loop-failure-contract
  mission: "As Lea, author a Loop failure contract in the Web editor, run it through recovery, and prove the published policy—not hidden defaults—determines the finish."
  mode: charter-with-tour
  persona:
    name: Lea
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-recover-loop-node-failure
  scenarios: [LP-editor-authoring-walk, LP-catalog-runform-walk, LP-transient-blip-heals, LP-error-route-fallback, LP-unannotated-escalation, LP-on-error-notification-with-context, LP-terminal-outcome-notification]
  tour: Feature Tour
  time_box_minutes: 90
  surfaces: [Web, CLI, HTTP, SSE]
  browser_plan: "Drive the UI with browser-use:browser (Playwright-backed); if unavailable, restart the session with agent-browser and record the fallback in the report."
  automated_precondition: "make test-e2e-runtime passes against the final build before the session starts."
  cross_surface_plan: "After every publish, start, recovery, and terminal checkpoint, compare the exact run and definition through structured CLI JSON and public HTTP; refresh or deep-link the Web page before accepting parity."
  evidence_expectations: [goal-state screenshots, every divergence screenshot, CLI JSON transcript, HTTP response body, SSE event excerpt, refresh confirmation]
  guidance:
    must_try:
      - "Author retry, on_error, node effects, terminal effects, wait exclusivity, and a result contract on a real custom Loop; warnings remain non-blocking while errors gate Publish."
      - "Publish and start from the matching catalog row; reject a blank required input without creating a run, then prove the Web definition equals the fresh HTTP definition."
      - "Trigger retry success, authored fallback, and unannotated repair on separate runs; each path must finish once and preserve classified context."
      - "Confirm on-error and terminal effects appear only after their owning state commits and never change that state."
    must_avoid:
      - "Fixtures, mocked 422 responses, SQLite reads, or source inspection as verdict evidence."
      - "Mobile editor assertions; the canvas is explicitly desktop-only and approval owns the mobile leg."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
