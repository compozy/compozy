# CH-extension-contract-policy: Close every declaration and configuration boundary

```yaml
charter:
  id: CH-extension-contract-policy
  mission: "As Vera, feed invalid and valid values through every extension config and declaration surface, proving one generated manifest v2, closed provides/permissions, truthful hook and command metadata, exact lifecycle reporting, and no permissive fallback."
  mode: charter-with-tour
  persona:
    name: Vera
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-extension-policy-admin
  scenarios: [ET-extension-manifest-v2-surfaces, ET-extension-legacy-key-rejection, ET-021, ET-044, ET-045]
  tour: Garbage Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Round-trip every named extensions.trust, extensions.sources, extensions.dev, and extensions.resources key through TOML, CLI, native config tools, HTTP/UDS settings, and Web where represented. Validate defaults and exact live/restart-required lifecycle."
      - "Use invalid URL, boolean, duration, resource kind, scope, and rate-limit values. Each rejection must preserve the prior applied value and identify the exact key without accepting an unknown extension config field."
      - "Build twice and byte-compare manifest v2. Add unknown provides/permissions, installed bridge.adapter, malformed command paths/groups/flags, and unsupported schema projections; build and load must fail before mutation with --input remediation where applicable."
      - "Load a valid extension hook and command tree. Hook introspection must report source extension and priority 300; command discovery must retain groups, leaves, risk, approval, and projected flags without granting execution authority."
    must_avoid:
      - "Hand-editing generated manifests, guessing wildcard config coverage, accepting an unknown declaration as ignored, or inferring command authority from presentation metadata."
  evidence_expectations:
    - "A key-by-key configuration matrix with default, mutation plane, validation result, lifecycle, and fresh read; deterministic manifest byte comparison."
    - "Closed-set rejection payloads plus valid hook/command projections across CLI, HTTP, UDS, native, and Web/settings owners."
```

## Selection rationale

Targeted tier. ADR-004, ADR-006, ADR-007, and ADR-008 own the permissions list, public bridge
boundary, consolidated config, and command metadata. Safety Invariants 6, 10–12, and 17 are explicit
targets. The exact key list in ET-045 prevents wildcard claims from hiding a missing config surface.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
