# CH-extension-agent-authoring: An autonomous agent completes the extension lifecycle without shell gaps

```yaml
charter:
  id: CH-extension-agent-authoring
  mission: "As Ada, activate the official Compozy skill and complete scaffold through removal with native tools and structured outputs only, proving that approvals, workspace scope, identity propagation, generated contracts, and credential redaction survive the complete agent-driven loop."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-extension-agent-authoring
  scenarios: [ET-extension-agent-guided-authoring, ET-compozy-official-skill-discovery, ET-extension-manifest-v2-surfaces]
  tour: Feature Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Start from the bundled `compozy` skill router and authoring reference. Call each new tool explicitly: extensions_init, build, validate, dev, reload, logs, search, provenance, and publish; record registration, risk class, approval behavior, and structured result."
      - "Complete scaffold -> build -> validate -> dev -> reload -> logs -> publish -> search -> install in a second workspace -> provenance -> invoke -> passive update -> update -> remove. Use canonical existing native tools for install/update/remove/invoke; never shell out."
      - "Pass an unknown provide, unknown permission, and installed bridge.adapter declaration. Each must fail closed at its owning boundary, while the four public provides remain representable by both SDK harnesses."
      - "Assert trusted_workspace and invocation_id inside Go and TypeScript handlers. Attempt cross-workspace dev/log access and inspect every transcript, diagnostic, event, SSE payload, and log for publish credentials or configured secrets."
    must_avoid:
      - "Browser or TTY fallback, hand-authored extension.toml, passing GITHUB_TOKEN in tool input, inferring approval from success, or treating bridge.adapter as a public provide."
  evidence_expectations:
    - "One ordered native-tool transcript with tool descriptors, approvals, structured results, workspace identities, and final removal state."
    - "Handler captures for trusted_workspace/invocation_id, closed-set rejection payloads, secret scans, and the SDK completeness/currency rubric inputs."
```

## Selection rationale

Targeted tier. ADR-001–ADR-007 converge here: code-first generation, the dev overlay, generated SDK
contracts, one permissions list, direct distribution, the bridge boundary, and consolidated config.
The box targets Safety Invariants 2–3, 6–14 and provides the required agent-driven official-skill
journey. It also owns the SDK completeness/currentness inputs needed for the A−/B/B re-grade.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
