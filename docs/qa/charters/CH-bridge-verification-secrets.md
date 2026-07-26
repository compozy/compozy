# CH-bridge-verification-secrets: Probe bridge verification and secret boundaries

```yaml
charter:
  id: CH-bridge-verification-secrets
  mission: "As Omar, attack the setup and verification trust boundaries with malformed credentials, private/redirecting callbacks, skipped identity providers, and secret-shaped output while proving probes remain read-only and redacted."
  mode: strategy-based
  persona:
    name: Omar
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-diagnose-repair-bridge
  scenarios: [NB-024, NB-029, NB-bridge-provider-setup]
  tour: Garbage Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Feed missing/wrong-shape slots and fake secret values containing sk-, Bearer, xoxb-, agh_claim_, PKCE-verifier, *_secret, and private-key markers; verify records, doctor output, errors, and wizard echoes must expose no raw value."
      - "Try provider_config API/OAuth/service destinations, localhost/private/link-local/mixed-DNS webhook URLs, redirects, and missing routes; expect deterministic rejection or truthful fail/warn without proxying credentials."
      - "Run verify repeatedly on disabled, ready, degraded, GitHub, and Linear instances; identity skipped must remain explicit and lifecycle state/revision must not change."
      - "Repair one named slot, enable, inspect runtime health, verify public reachability, and compare per-instance records with doctor aggregation."
    must_avoid:
      - "Real secrets, external untrusted hosts, interpreting skipped as pass, or changing lifecycle through anything other than explicit operator actions."
  evidence_expectations:
    - "Redacted CLI/HTTP/UDS outputs, lifecycle snapshots before/after probes, rejected URL matrix, no-proxy fake-server log, enabled runtime health, and doctor comparison."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
