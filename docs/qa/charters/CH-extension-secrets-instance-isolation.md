# CH-extension-secrets-instance-isolation: Keep extension secrets inside their owning instance

```yaml
charter:
  id: CH-extension-secrets-instance-isolation
  mission: "As Bruno, stress extension secret set, bind, list, injection, rotation, update, unset, and retirement across one global and two workspace instances, proving no value or authority crosses an instance or public surface."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-extension-kit-lifecycle
  scenarios: [ET-ext-secrets-binding]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Use the active cycle's fresh bootstrap envelope and its exact global/workspace identities; run config and binding writes sequentially and never read or write the operator's default home."
      - "Set one value through hidden input/stdin and bind another existing extensions-namespace Vault ref; prove list, status, doctor, HTTP, UDS, CLI json/jsonl/toon, logs, events, SSE, and tool transcripts expose key names or redaction only."
      - "Probe value+ref overlap, undeclared env, dangling ref, wrong namespace, stale declaration after update, empty and unusually long pasted input, and an injected rollback failure. Every 400/500 path preserves the prior bindings and values in reverse-order rollback."
      - "Bind homonymous keys for global, workspace A, and workspace B; launch each instance and prove exact injection with no global fallback, sibling read, sibling confirmation, or cross-workspace cache/event leakage."
      - "Rotate and update the same managed identity, then unset and remove. Bindings survive update, stale bindings never inject, owned extension_env refs GC only when unreferenced, foreign-kind refs remain, and reinstall starts with no inherited authority."
      - "Confirm the native catalog exposes no secret-write tool; an agent uses the documented CLI/UDS path and still receives deterministic structured results."
    must_avoid:
      - "Printing test secret plaintext into argv, evidence, reports, prompts, or terminal transcripts; using a production credential; widening the test to MCP or bridge secret ownership."
```

## Evidence expectations

- Presence-only cross-instance matrix, spawn observables with redacted values, rollback and GC reads,
  deterministic error payloads, and a corpus-wide plaintext scan with zero hits.

## Selection rationale

Targeted tier owner for Safety Invariants 5–7 and 17 and ADR-003.

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
