# CH-secret-redaction-sweep: Prove no planted secret survives any durable or streamed surface

```yaml
charter:
  id: CH-secret-redaction-sweep
  mission: "As Dora, plant provider-shaped and exact-class secrets through agent output and tool input, then grep every durable store and stream to prove only the redaction marker exists while correlation envelopes survive intact."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-keep-secrets-contained
  scenarios: [RT-secret-redaction-boundary]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Emit one unique provider-shaped fixture secret via an assistant response AND a tool input; grep logs, SSE capture, runtime.db dump, and the session events.db dump — zero raw hits (SD-006 timestamps + commands)."
      - "Verify claim_token_hash, session/run ids, and fingerprints survive byte-identical in the same records; pass a code-heavy non-secret fixture through and require byte-identical output."
      - "Flip redact.enabled=false via a public config surface — the mutation must report restart-required and the boot snapshot must hold until restart; after restart, exact claim-token and registered-secret protections must still redact."
      - "Confirm SECURITY.md renders on the site and its BackendLocal/stdio-authority claims match shipped behavior."
    must_avoid:
      - "Real credentials — fixture secrets only."
      - "Treating a UI-only check as evidence: every claim needs the store/stream grep."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
