# CH-worktree-forge-credential-boundary: Assisted exit works without exporting Forge authority

```yaml
charter:
  id: CH-worktree-forge-credential-boundary
  mission: "As Dora, exercise the GitHub forge provider with bound, gh-sourced, expired, unsupported, and absent credentials while proving the daemon and every durable surface receive capability and result facts but never a token."
  mode: strategy-based
  persona:
    name: Dora
    device: desktop
    network: flaky
    locale: en-US
  journey: J-worktree-management
  scenarios: [ET-worktree-forge-provider-boundary, RT-worktree-exit-pr-idempotency, RT-worktree-exit-browser-fallback, RT-worktree-web-exit-commit-pr]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Use unique fixture tokens for extension secret binding and gh auth fallback, verify binding wins, and grep daemon logs, extension-safe output, HTTP/UDS payloads, SSE, events, runtime databases, and memory for zero raw hits."
      - "Probe forge/capabilities, forge/status, and forge/pr_create through a GitHub remote; verify provider vocabulary, draft support, base/template resolution, status attribution, and idempotent reuse of an existing request."
      - "Return rate_limited, credential_expired, unsupported_remote, and credential_absent, then disable the provider; each state must degrade truthfully without retries, invented PR controls, or loss of local commit/push actions."
      - "With no credential, verify the Web and structured exit plan expose only the sanitized compare or remote-root browser action for known and unknown hosts."
    must_avoid:
      - "Real credentials, public repository mutations, or accepting a token-shaped value in captured evidence."
      - "Calling the marketplace GitHub MCP as proof of forge.provider behavior; the surfaces are intentionally independent."
  coverage:
    tier: targeted
    surfaces: [forge.provider, GitHub-extension, secret-binding, gh-CLI, CLI, HTTP, UDS, web-S6-S14, SSE, durable-events]
    invariants: [16]
    hot_spots:
      - "Credential authority must remain inside the extension process even when capability and status facts cross the daemon boundary."
      - "Provider failure and absence must remove only Forge-backed affordances and preserve the zero-credential browser tier."
    adrs: [ADR-004, ADR-010]
    expected_evidence: "Credential-source matrices, provider responses, sanitized browser links, idempotent request reads, and zero-hit redaction scans."
    exit_criteria: "Every credential tier reports the correct safe capability state, request creation is idempotent, fallback remains usable, and no fixture token escapes."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief belongs in that run's dated report. -->
